package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewMessageProcessor(t *testing.T) {
	rawChan := make(chan []byte, 10)
	processedChan := make(chan *Message, 10)
	logger := NewLogger(false)

	processor := NewMessageProcessor(rawChan, processedChan, logger)

	if processor == nil {
		t.Fatal("Expected processor to be created, got nil")
	}
}

func TestMessageProcessor_ProcessMessages(t *testing.T) {
	rawChan := make(chan []byte, 10)
	processedChan := make(chan *Message, 10)
	logger := NewLogger(false)

	processor := NewMessageProcessor(rawChan, processedChan, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start processing in goroutine
	go processor.ProcessMessages(ctx)

	// Send test message
	testMessage := []byte(`{"id": "test123", "type": "post", "content": "Hello World"}`)
	rawChan <- testMessage

	// Wait for processed message
	select {
	case processed := <-processedChan:
		if processed.ID != "test123" {
			t.Errorf("Expected ID 'test123', got '%s'", processed.ID)
		}
		if processed.Type != "post" {
			t.Errorf("Expected type 'post', got '%s'", processed.Type)
		}
		if processed.Data["content"] != "Hello World" {
			t.Errorf("Expected content 'Hello World', got '%v'", processed.Data["content"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for processed message")
	}

	close(rawChan)
}

func TestMessageProcessor_InvalidJSON(t *testing.T) {
	rawChan := make(chan []byte, 10)
	processedChan := make(chan *Message, 10)
	logger := NewLogger(false)

	processor := NewMessageProcessor(rawChan, processedChan, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go processor.ProcessMessages(ctx)

	// Send invalid JSON
	invalidMessage := []byte(`{"invalid": json}`)
	rawChan <- invalidMessage

	// Should not receive any processed message
	select {
	case <-processedChan:
		t.Fatal("Expected no processed message for invalid JSON")
	case <-time.After(100 * time.Millisecond):
		// Expected - no message should be processed
	}

	close(rawChan)
}

func TestProcessRawMessage_ValidJSON(t *testing.T) {
	rawChan := make(chan []byte, 10)
	processedChan := make(chan *Message, 10)
	logger := NewLogger(false)

	processor := NewMessageProcessor(rawChan, processedChan, logger)

	testJSON := []byte(`{"id": "test456", "type": "like", "target": "post123"}`)
	message, err := processor.processRawMessage(testJSON)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if message.ID != "test456" {
		t.Errorf("Expected ID 'test456', got '%s'", message.ID)
	}

	if message.Type != "like" {
		t.Errorf("Expected type 'like', got '%s'", message.Type)
	}

	if message.Data["target"] != "post123" {
		t.Errorf("Expected target 'post123', got '%v'", message.Data["target"])
	}

	if message.Timestamp == 0 {
		t.Error("Expected timestamp to be set")
	}
}

func TestProcessRawMessage_InvalidJSON(t *testing.T) {
	rawChan := make(chan []byte, 10)
	processedChan := make(chan *Message, 10)
	logger := NewLogger(false)

	processor := NewMessageProcessor(rawChan, processedChan, logger)

	invalidJSON := []byte(`{"invalid": json`)
	_, err := processor.processRawMessage(invalidJSON)

	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Expected unmarshal error, got: %v", err)
	}
}

func TestGenerateMessageID(t *testing.T) {
	// Test with existing ID
	data1 := map[string]interface{}{"id": "existing123"}
	id1 := generateMessageID(data1)
	if id1 != "existing123" {
		t.Errorf("Expected 'existing123', got '%s'", id1)
	}

	// Test with CID
	data2 := map[string]interface{}{"cid": "cid456"}
	id2 := generateMessageID(data2)
	if id2 != "cid456" {
		t.Errorf("Expected 'cid456', got '%s'", id2)
	}

	// Test with URI
	data3 := map[string]interface{}{"uri": "at://example.com/post/123"}
	id3 := generateMessageID(data3)
	if id3 != "at://example.com/post/123" {
		t.Errorf("Expected URI, got '%s'", id3)
	}

	// Test fallback
	data4 := map[string]interface{}{"other": "data"}
	id4 := generateMessageID(data4)
	if !strings.HasPrefix(id4, "msg_") {
		t.Errorf("Expected fallback ID to start with 'msg_', got '%s'", id4)
	}
}

func TestMessageProcessor_ContextCancellation(t *testing.T) {
	rawChan := make(chan []byte, 10)
	processedChan := make(chan *Message, 10)
	logger := NewLogger(false)

	processor := NewMessageProcessor(rawChan, processedChan, logger)

	ctx, cancel := context.WithCancel(context.Background())

	// Start processing
	go processor.ProcessMessages(ctx)

	// Cancel context immediately
	cancel()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// Should stop gracefully without hanging
}

func TestMessageProcessor_ChannelClose(t *testing.T) {
	rawChan := make(chan []byte, 10)
	processedChan := make(chan *Message, 10)
	logger := NewLogger(false)

	processor := NewMessageProcessor(rawChan, processedChan, logger)

	ctx := context.Background()

	// Start processing
	go processor.ProcessMessages(ctx)

	// Close input channel
	close(rawChan)

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// Should stop gracefully without hanging
}