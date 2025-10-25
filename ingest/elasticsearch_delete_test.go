package main

import (
	"strings"
	"testing"
	"time"
)

func TestDeleteMessageFlow(t *testing.T) {
	logger := NewLogger(false)

	postAtURI := "at://did:plc:test123/app.bsky.feed.post/abc123"
	postDID := "did:plc:test123"
	postContent := "Hello, world!"
	postCreatedAt := time.Now().UTC().Format(time.RFC3339)

	postRawJSON := `{
		"message": {
			"commit": {
				"operation": "create",
				"record": {
					"text": "` + postContent + `",
					"createdAt": "` + postCreatedAt + `"
				}
			}
		}
	}`

	deleteRawJSON := `{
		"message": {
			"commit": {
				"operation": "delete"
			}
		}
	}`

	postMsg := NewMegaStreamMessage(postAtURI, postDID, postRawJSON, "{}", logger)
	if postMsg.IsDelete() {
		t.Error("Expected post message to not be a delete")
	}

	postDoc := CreateElasticsearchDoc(postMsg)
	if postDoc.AtURI != postAtURI {
		t.Errorf("Expected AtURI %s, got %s", postAtURI, postDoc.AtURI)
	}
	if postDoc.AuthorDID != postDID {
		t.Errorf("Expected AuthorDID %s, got %s", postDID, postDoc.AuthorDID)
	}
	if postDoc.Content != postContent {
		t.Errorf("Expected Content %s, got %s", postContent, postDoc.Content)
	}
	if postDoc.IndexedAt == "" {
		t.Error("Expected post to have IndexedAt timestamp")
	}

	_, err := time.Parse(time.RFC3339, postDoc.IndexedAt)
	if err != nil {
		t.Errorf("Expected IndexedAt to be valid RFC3339 timestamp, got error: %v", err)
	}

	deleteMsg := NewMegaStreamMessage(postAtURI, postDID, deleteRawJSON, "{}", logger)
	if !deleteMsg.IsDelete() {
		t.Error("Expected delete message to be a delete")
	}

	tombstoneDoc := CreateTombstoneDoc(deleteMsg)
	if tombstoneDoc.AtURI != postAtURI {
		t.Errorf("Expected tombstone AtURI %s, got %s", postAtURI, tombstoneDoc.AtURI)
	}
	if tombstoneDoc.AuthorDID != postDID {
		t.Errorf("Expected tombstone AuthorDID %s, got %s", postDID, tombstoneDoc.AuthorDID)
	}
	if tombstoneDoc.DeletedAt == "" {
		t.Error("Expected tombstone to have DeletedAt timestamp")
	}

	_, err = time.Parse(time.RFC3339, tombstoneDoc.DeletedAt)
	if err != nil {
		t.Errorf("Expected tombstone DeletedAt to be valid RFC3339 timestamp, got error: %v", err)
	}
}

func TestCreateTombstoneDoc(t *testing.T) {
	logger := NewLogger(false)

	atURI := "at://did:plc:abc/app.bsky.feed.post/xyz"
	did := "did:plc:abc"

	deleteRawJSON := `{
		"message": {
			"commit": {
				"operation": "delete"
			}
		}
	}`

	msg := NewMegaStreamMessage(atURI, did, deleteRawJSON, "{}", logger)

	if !msg.IsDelete() {
		t.Fatal("Expected message to be a delete")
	}

	tombstone := CreateTombstoneDoc(msg)

	if tombstone.AtURI != atURI {
		t.Errorf("Expected AtURI %s, got %s", atURI, tombstone.AtURI)
	}

	if tombstone.AuthorDID != did {
		t.Errorf("Expected AuthorDID %s, got %s", did, tombstone.AuthorDID)
	}

	if tombstone.DeletedAt == "" {
		t.Error("Expected DeletedAt to be set")
	}

	deletedAt, err := time.Parse(time.RFC3339, tombstone.DeletedAt)
	if err != nil {
		t.Errorf("Expected DeletedAt to be valid RFC3339, got error: %v", err)
	}

	if time.Since(deletedAt) > time.Second {
		t.Errorf("Expected DeletedAt to be recent, got %v", deletedAt)
	}
}

func TestTombstoneDocFields(t *testing.T) {
	logger := NewLogger(false)

	atURI := "at://did:plc:test/app.bsky.feed.post/123"
	did := "did:plc:test"

	deleteJSON := `{"message":{"commit":{"operation":"delete"}}}`
	msg := NewMegaStreamMessage(atURI, did, deleteJSON, "{}", logger)

	tombstone := CreateTombstoneDoc(msg)

	if !strings.HasPrefix(atURI, "at://") {
		t.Error("Test data should have valid at_uri format")
	}

	if tombstone.AtURI != msg.GetAtURI() {
		t.Error("Tombstone AtURI should match message AtURI")
	}

	if tombstone.AuthorDID != msg.GetAuthorDID() {
		t.Error("Tombstone AuthorDID should match message AuthorDID")
	}

	if tombstone.DeletedAt == "" {
		t.Error("Tombstone must have DeletedAt timestamp")
	}
}

func TestDeleteMessage_IsDelete(t *testing.T) {
	logger := NewLogger(false)

	tests := []struct {
		name       string
		rawJSON    string
		wantDelete bool
	}{
		{
			name: "delete operation",
			rawJSON: `{
				"message": {
					"commit": {
						"operation": "delete"
					}
				}
			}`,
			wantDelete: true,
		},
		{
			name: "create operation",
			rawJSON: `{
				"message": {
					"commit": {
						"operation": "create",
						"record": {
							"text": "test post"
						}
					}
				}
			}`,
			wantDelete: false,
		},
		{
			name:       "empty json",
			rawJSON:    `{}`,
			wantDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMegaStreamMessage("at://test", "did:test", tt.rawJSON, "{}", logger)
			if msg.IsDelete() != tt.wantDelete {
				t.Errorf("IsDelete() = %v, want %v", msg.IsDelete(), tt.wantDelete)
			}
		})
	}
}

func TestBulkOperations_DryRun(t *testing.T) {
	logger := NewLogger(false)

	t.Run("bulkIndexTombstones dry-run returns no error", func(t *testing.T) {
		tombstone := TombstoneDoc{
			AtURI:     "at://did:plc:test/app.bsky.feed.post/123",
			AuthorDID: "did:plc:test",
			DeletedAt: time.Now().UTC().Format(time.RFC3339),
		}

		err := bulkIndexTombstones(nil, nil, "post_tombstones", []TombstoneDoc{tombstone}, true, logger)
		if err != nil {
			t.Errorf("Expected no error in dry-run mode, got: %v", err)
		}
	})

	t.Run("bulkDelete dry-run returns no error", func(t *testing.T) {
		err := bulkDelete(nil, nil, "posts", []string{"at://did:plc:test/app.bsky.feed.post/123"}, true, logger)
		if err != nil {
			t.Errorf("Expected no error in dry-run mode, got: %v", err)
		}
	})
}

func TestBulkOperations_EmptyBatch(t *testing.T) {
	logger := NewLogger(false)

	t.Run("bulkIndexTombstones empty batch returns no error", func(t *testing.T) {
		err := bulkIndexTombstones(nil, nil, "post_tombstones", []TombstoneDoc{}, false, logger)
		if err != nil {
			t.Errorf("Expected no error for empty batch, got: %v", err)
		}
	})

	t.Run("bulkDelete empty batch returns no error", func(t *testing.T) {
		err := bulkDelete(nil, nil, "posts", []string{}, false, logger)
		if err != nil {
			t.Errorf("Expected no error for empty batch, got: %v", err)
		}
	})
}
