package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TODO: Use this multithreaded pipeline implementation in main.go
// The current single-threaded SQLite processing in main.go should be refactored
// to use this channel-based MessageProcessor architecture for concurrent processing
// of messages from multiple data sources (WebSocket, local SQLite, S3-hosted SQLite)

// MessageProcessor handles the processing of individual messages
type MessageProcessor struct {
	rawMessageChan       <-chan []byte
	processedMessageChan chan<- *Message
	logger               Logger
}

// NewMessageProcessor creates a new message processor with existing channels
func NewMessageProcessor(
	rawChan <-chan []byte,
	processedChan chan<- *Message,
	logger Logger,
) *MessageProcessor {
	return &MessageProcessor{
		rawMessageChan:       rawChan,
		processedMessageChan: processedChan,
		logger:               logger,
	}
}

// ProcessMessages processes raw messages from the WebSocket into structured messages
func (mp *MessageProcessor) ProcessMessages(ctx context.Context) {
	mp.logger.Info("Starting message processor")

	for {
		select {
		case <-ctx.Done():
			mp.logger.Info("Message processor stopped due to context cancellation")
			return
		case rawMessage, ok := <-mp.rawMessageChan:
			if !ok {
				mp.logger.Info("Raw message channel closed, stopping message processor")
				return
			}

			message, err := mp.processRawMessage(rawMessage)
			if err != nil {
				mp.logger.Error("Failed to process message: %v", err)
				continue
			}

			select {
			case mp.processedMessageChan <- message:
				mp.logger.Debug("Processed message: %s", message.ID)
			case <-ctx.Done():
				mp.logger.Info("Message processor stopped during message send")
				return
			default:
				mp.logger.Error("Processed message channel full, dropping message: %s", message.ID)
			}
		}
	}
}

// processRawMessage converts a raw JSON message into a structured Message
func (mp *MessageProcessor) processRawMessage(rawMessage []byte) (*Message, error) {
	var rawData map[string]interface{}
	if err := json.Unmarshal(rawMessage, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Extract basic fields
	messageType, _ := rawData["type"].(string)
	if messageType == "" {
		messageType = "unknown"
	}

	// Generate ID if not present
	messageID := generateMessageID(rawData)

	// Create structured message
	message := &Message{
		ID:        messageID,
		Type:      messageType,
		Data:      rawData,
		Timestamp: time.Now().Unix(),
	}

	return message, nil
}

// generateMessageID generates a unique ID for a message based on its content
func generateMessageID(data map[string]interface{}) string {
	// Try to use existing ID fields
	if id, ok := data["id"].(string); ok && id != "" {
		return id
	}
	if id, ok := data["cid"].(string); ok && id != "" {
		return id
	}
	if uri, ok := data["uri"].(string); ok && uri != "" {
		return uri
	}

	// Fallback to timestamp-based ID
	return fmt.Sprintf("msg_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}