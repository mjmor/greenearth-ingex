package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
)

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

// ElasticsearchConfig holds configuration for Elasticsearch connection
type ElasticsearchConfig struct {
	URL           string
	APIKey        string
	SkipTLSVerify bool
}

// NewElasticsearchClient creates and tests a new Elasticsearch client
func NewElasticsearchClient(config ElasticsearchConfig, logger *IngestLogger) (*elasticsearch.Client, error) {
	esConfig := elasticsearch.Config{
		Addresses: []string{config.URL},
		APIKey:    config.APIKey,
	}

	if config.SkipTLSVerify {
		logger.Info("TLS certificate verification disabled (local development mode)")
		esConfig.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Elasticsearch: %w", err)
	}
	res.Body.Close()

	logger.Info("Connected to Elasticsearch at %s", config.URL)
	return client, nil
}

// bulkIndex indexes a batch of documents to Elasticsearch
func bulkIndex(ctx context.Context, client *elasticsearch.Client, index string, docs []ElasticsearchDoc, dryRun bool, logger *IngestLogger) error {
	if len(docs) == 0 {
		return nil
	}

	if dryRun {
		logger.Debug("Dry-run: Skipping bulk index of %d documents to index '%s'", len(docs), index)
		return nil
	}

	var buf bytes.Buffer

	for _, doc := range docs {
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

		docJSON, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("failed to marshal document: %w", err)
		}

		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

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

	if bulkResponse.Errors {
		itemsJSON, _ := json.Marshal(bulkResponse.Items)
		logger.Error("Bulk indexing failed with errors. Response items: %s", string(itemsJSON))
		return fmt.Errorf("bulk indexing failed: some documents had errors (see logs for details)")
	}

	return nil
}

// CreateElasticsearchDoc creates an ElasticsearchDoc from a MegaStreamMessage
func CreateElasticsearchDoc(msg MegaStreamMessage) ElasticsearchDoc {
	return ElasticsearchDoc{
		AtURI:            msg.GetAtURI(),
		AuthorDID:        msg.GetAuthorDID(),
		Content:          msg.GetContent(),
		CreatedAt:        msg.GetCreatedAt(),
		ThreadRootPost:   msg.GetThreadRootPost(),
		ThreadParentPost: msg.GetThreadParentPost(),
		QuotePost:        msg.GetQuotePost(),
		Embeddings:       msg.GetEmbeddings(),
		IndexedAt:        time.Now().UTC().Format(time.RFC3339),
	}
}
