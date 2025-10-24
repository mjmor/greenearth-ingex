package main

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"os/signal"
	"syscall"

	_ "modernc.org/sqlite"
)

// TODO: Move to multithreaded implementation

func main() {
	// Parse command line flags
	dryRun := flag.Bool("dry-run", false, "Run in dry-run mode (no writes to Elasticsearch)")
	skipTLSVerify := flag.Bool("skip-tls-verify", false, "Skip TLS certificate verification (use for local development only)")
	flag.Parse()

	// Load configuration
	config := LoadConfig()
	logger := NewLogger(config.LoggingEnabled)

	logger.Info("Green Earth Ingex - BlueSky Ingest Service")
	if *dryRun {
		logger.Info("Running in DRY-RUN mode - no writes to Elasticsearch")
	}
	logger.Info("Starting SQLite ingestion from Megastream database")

	// Validate configuration
	if config.SQLiteDBPath == "" {
		logger.Error("SQLITE_DB_PATH environment variable is required")
		os.Exit(1)
	}

	if config.ElasticsearchURL == "" {
		logger.Error("ELASTICSEARCH_URL environment variable is required")
		os.Exit(1)
	}

	if !*dryRun && config.ElasticsearchAPIKey == "" {
		logger.Error("ELASTICSEARCH_API_KEY environment variable is required")
		os.Exit(1)
	}

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, finishing current batch...")
		cancel()
	}()

	// Initialize Elasticsearch client
	esConfig := ElasticsearchConfig{
		URL:           config.ElasticsearchURL,
		APIKey:        config.ElasticsearchAPIKey,
		SkipTLSVerify: *skipTLSVerify,
	}

	esClient, err := NewElasticsearchClient(esConfig, logger)
	if err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}

	// Open SQLite database
	db, err := sql.Open("sqlite", config.SQLiteDBPath)
	if err != nil {
		logger.Error("Failed to open SQLite database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("Opened SQLite database: %s", config.SQLiteDBPath)

	// Query enriched_posts table
	rows, err := db.QueryContext(ctx, `
		SELECT at_uri, did, raw_post, inferences
		FROM enriched_posts
	`)
	if err != nil {
		logger.Error("Failed to query enriched_posts: %v", err)
		os.Exit(1)
	}
	defer rows.Close()

	// Process rows and bulk index to Elasticsearch
	var batch []ElasticsearchDoc
	const batchSize = 100
	processedCount := 0
	skippedCount := 0

	for rows.Next() {
		select {
		case <-ctx.Done():
			logger.Info("Shutdown signal received, stopping ingestion")
			goto cleanup
		default:
		}

		var atURI, did, rawPostJSON, inferencesJSON string
		if err := rows.Scan(&atURI, &did, &rawPostJSON, &inferencesJSON); err != nil {
			logger.Error("Failed to scan row: %v", err)
			continue
		}

		msg := NewMegaStreamMessage(atURI, did, rawPostJSON, inferencesJSON, logger)

		// TODO: Handle post deletions in Elasticsearch instead of skipping
		// We should delete or mark as deleted in ES when operation == "delete"
		if msg.IsDelete() {
			skippedCount++
			continue
		}

		doc := CreateElasticsearchDoc(msg)

		batch = append(batch, doc)

		// Bulk index when batch is full
		if len(batch) >= batchSize {
			if err := bulkIndex(ctx, esClient, "posts", batch, *dryRun, logger); err != nil {
				logger.Error("Failed to bulk index batch: %v", err)
			} else {
				processedCount += len(batch)
				if *dryRun {
					logger.Info("Dry-run: Would index batch: %d documents (total: %d, skipped: %d)", len(batch), processedCount, skippedCount)
				} else {
					logger.Info("Indexed batch: %d documents (total: %d, skipped: %d)", len(batch), processedCount, skippedCount)
				}
			}
			batch = batch[:0]
		}
	}

cleanup:
	// Index remaining documents in batch
	if len(batch) > 0 {
		if err := bulkIndex(ctx, esClient, "posts", batch, *dryRun, logger); err != nil {
			logger.Error("Failed to bulk index final batch: %v", err)
		} else {
			processedCount += len(batch)
			if *dryRun {
				logger.Info("Dry-run: Would index final batch: %d documents", len(batch))
			} else {
				logger.Info("Indexed final batch: %d documents", len(batch))
			}
		}
	}

	if err := rows.Err(); err != nil {
		logger.Error("Error iterating rows: %v", err)
		os.Exit(1)
	}

	logger.Info("Ingestion complete. Processed: %d, Skipped: %d", processedCount, skippedCount)
}
