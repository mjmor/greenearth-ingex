package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	_ "modernc.org/sqlite"
)

// TODO: Move to multithreaded implementation with DataSource interface abstraction
// This will support multiple data sources: WebSocket, local SQLite, and S3-hosted SQLite

// ElasticsearchDoc represents the document structure for indexing
type ElasticsearchDoc struct {
	AtURI            string               `json:"at_uri"`
	AuthorDID        string               `json:"author_did"`
	Content          string               `json:"content"`
	CreatedAt        string               `json:"created_at"`
	ThreadRootPost   string               `json:"thread_root_post,omitempty"`
	ThreadParentPost string               `json:"thread_parent_post,omitempty"`
	QuotePost        string               `json:"quote_post,omitempty"`
	Embeddings       map[string][]float32 `json:"embeddings,omitempty"`
	IndexedAt        string               `json:"indexed_at"`
}

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
	esConfig := elasticsearch.Config{
		Addresses: []string{config.ElasticsearchURL},
		APIKey:    config.ElasticsearchAPIKey,
	}

	// Configure TLS settings if skip verification is requested
	if *skipTLSVerify {
		logger.Info("TLS certificate verification disabled (local development mode)")
		esConfig.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	esClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		logger.Error("Failed to create Elasticsearch client: %v", err)
		os.Exit(1)
	}

	// Test Elasticsearch connection
	res, err := esClient.Info()
	if err != nil {
		logger.Error("Failed to connect to Elasticsearch: %v", err)
		os.Exit(1)
	}
	res.Body.Close()
	logger.Info("Connected to Elasticsearch at %s", config.ElasticsearchURL)

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

		// TODO: Create common implementation for message structure and deserialization
		// This should replace the JSON parsing code in both pipeline.go and main.go

		// Parse raw_post JSON
		var rawPost map[string]interface{}
		if err := json.Unmarshal([]byte(rawPostJSON), &rawPost); err != nil {
			logger.Error("Failed to parse raw_post JSON for %s: %v", atURI, err)
			continue
		}

		// Check for delete operation
		message, ok := rawPost["message"].(map[string]interface{})
		if !ok {
			logger.Debug("No message field in raw_post for %s", atURI)
			continue
		}

		commit, ok := message["commit"].(map[string]interface{})
		if !ok {
			logger.Debug("No commit field in message for %s", atURI)
			continue
		}

		operation, _ := commit["operation"].(string)
		// TODO: Handle post deletions in Elasticsearch instead of skipping
		// We should delete or mark as deleted in ES when operation == "delete"
		if operation == "delete" {
			skippedCount++
			continue
		}

		// Extract record data
		record, ok := commit["record"].(map[string]interface{})
		if !ok {
			logger.Debug("No record field in commit for %s", atURI)
			continue
		}

		// Extract content
		content, _ := record["text"].(string)

		// Extract createdAt
		createdAt, _ := record["createdAt"].(string)

		// Parse hydrated_metadata for thread/quote info
		hydratedMetadata, _ := rawPost["hydrated_metadata"].(map[string]interface{})

		var threadRootPost, threadParentPost, quotePost string

		if hydratedMetadata != nil {
			if replyPost, ok := hydratedMetadata["reply_post"].(map[string]interface{}); ok {
				threadRootPost, _ = replyPost["uri"].(string)
			}

			if parentPost, ok := hydratedMetadata["parent_post"].(map[string]interface{}); ok {
				threadParentPost, _ = parentPost["uri"].(string)
			}

			if qPost, ok := hydratedMetadata["quote_post"].(map[string]interface{}); ok {
				quotePost, _ = qPost["uri"].(string)
			}
		}

		// Parse inferences JSON for embeddings
		var inferences map[string]interface{}
		embeddings := make(map[string][]float32)

		if err := json.Unmarshal([]byte(inferencesJSON), &inferences); err == nil {
			if textEmbeddings, ok := inferences["text_embeddings"].(map[string]interface{}); ok {
				// Decode all-MiniLM-L12-v2 embeddings
				if embL12, ok := textEmbeddings["all-MiniLM-L12-v2"].(string); ok {
					if decoded, err := decodeEmbedding(embL12); err == nil {
						embeddings["all_MiniLM_L12_v2"] = decoded
					} else {
						logger.Debug("Failed to decode L12 embedding for %s: %v", atURI, err)
					}
				}

				// Decode all-MiniLM-L6-v2 embeddings
				if embL6, ok := textEmbeddings["all-MiniLM-L6-v2"].(string); ok {
					if decoded, err := decodeEmbedding(embL6); err == nil {
						embeddings["all_MiniLM_L6_v2"] = decoded
					} else {
						logger.Debug("Failed to decode L6 embedding for %s: %v", atURI, err)
					}
				}
			}
		}

		// Create Elasticsearch document
		doc := ElasticsearchDoc{
			AtURI:            atURI,
			AuthorDID:        did,
			Content:          content,
			CreatedAt:        createdAt,
			ThreadRootPost:   threadRootPost,
			ThreadParentPost: threadParentPost,
			QuotePost:        quotePost,
			Embeddings:       embeddings,
			IndexedAt:        time.Now().UTC().Format(time.RFC3339),
		}

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

// decodeEmbedding decodes a base64-encoded embedding string to float32 array
func decodeEmbedding(encoded string) ([]float32, error) {
	// TODO: The embeddings appear to be custom encoded, not standard base64;
	// we need to check with Graze for appropriate encoding function.
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	// Convert bytes to float32 array
	floatCount := len(decoded) / 4
	floats := make([]float32, floatCount)

	for i := range floatCount {
		bits := binary.LittleEndian.Uint32(decoded[i*4 : (i+1)*4])
		floats[i] = float32(bits)
	}

	return floats, nil
}

// TODO: Move Elasticsearch indexing code to separate file implementing ElasticsearchClient from interfaces.go
// This will allow for better separation of concerns and easier testing

// bulkIndex indexes a batch of documents to Elasticsearch
func bulkIndex(ctx context.Context, client *elasticsearch.Client, index string, docs []ElasticsearchDoc, dryRun bool, logger *IngestLogger) error {
	if len(docs) == 0 {
		return nil
	}

	// In dry-run mode, skip the actual write operation
	if dryRun {
		logger.Debug("Dry-run: Skipping bulk index of %d documents to index '%s'", len(docs), index)
		return nil
	}

	var buf bytes.Buffer

	for _, doc := range docs {
		// Add index action
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": index,
				"_id":    doc.AtURI,
			},
		}

		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		buf.Write(metaJSON)
		buf.WriteByte('\n')

		// Add document
		docJSON, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("failed to marshal document: %w", err)
		}

		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	// Perform bulk request
	res, err := client.Bulk(
		bytes.NewReader(buf.Bytes()),
		client.Bulk.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("bulk request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk request returned error: %s", res.String())
	}

	// Parse the response to check for individual document errors
	var bulkResponse struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Error *struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"error"`
		} `json:"items"`
	}

	if err := json.NewDecoder(res.Body).Decode(&bulkResponse); err != nil {
		return fmt.Errorf("failed to parse bulk response: %w", err)
	}

	// If any documents had errors, log details and return error
	if bulkResponse.Errors {
		itemsJSON, _ := json.Marshal(bulkResponse.Items)
		logger.Error("Bulk indexing failed with errors. Response items: %s", string(itemsJSON))
		return fmt.Errorf("bulk indexing failed: some documents had errors (see logs for details)")
	}

	return nil
}
