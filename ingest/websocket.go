package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// TODO: Abstract WebSocketClient interface to a general DataSource interface
// Create separate implementations for:
// 1. WebSocketDataSource - for real-time websocket streams (this file)
// 2. LocalSQLiteDataSource - for local SQLite file ingestion
// 3. S3SQLiteDataSource - for remote SQLite files hosted on S3
// All implementations should provide a common interface for reading messages

// TurboStreamClient implements the WebSocketClient interface for TurboStream connections
type TurboStreamClient struct {
	conn   *websocket.Conn
	logger Logger
}

// NewTurboStreamClient creates a new TurboStream WebSocket client
func NewTurboStreamClient(logger Logger) *TurboStreamClient {
	return &TurboStreamClient{
		logger: logger,
	}
}

// Connect establishes a WebSocket connection to the TurboStream URL
func (c *TurboStreamClient) Connect(ctx context.Context, url string) error {
	c.logger.Info("Connecting to TurboStream at %s", url)

	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, url, http.Header{
		"User-Agent": []string{"greenearth-ingex/1.0"},
	})

	if err != nil {
		if resp != nil {
			c.logger.Error("WebSocket connection failed with status %d: %v", resp.StatusCode, err)
		} else {
			c.logger.Error("WebSocket connection failed: %v", err)
		}
		return fmt.Errorf("failed to connect to TurboStream: %w", err)
	}

	if resp != nil {
		resp.Body.Close()
	}

	c.conn = conn
	c.logger.Info("Successfully connected to TurboStream")
	return nil
}

// ReadMessage reads the next message from the WebSocket connection
func (c *TurboStreamClient) ReadMessage(ctx context.Context) ([]byte, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("websocket connection not established")
	}

	// Set read deadline based on context
	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetReadDeadline(deadline)
	}

	messageType, message, err := c.conn.ReadMessage()
	if err != nil {
		c.logger.Error("Failed to read WebSocket message: %v", err)
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		c.logger.Debug("Received non-data message type: %d", messageType)
		return nil, fmt.Errorf("received non-data message type: %d", messageType)
	}

	c.logger.Debug("Received message of %d bytes", len(message))
	return message, nil
}

// Close closes the WebSocket connection
func (c *TurboStreamClient) Close() error {
	if c.conn == nil {
		return nil
	}

	c.logger.Info("Closing WebSocket connection")

	// Send close message with a timeout
	closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	err := c.conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(time.Second))
	if err != nil {
		c.logger.Error("Failed to send close message: %v", err)
	}

	// Close the connection
	closeErr := c.conn.Close()
	if closeErr != nil {
		c.logger.Error("Failed to close WebSocket connection: %v", closeErr)
		return closeErr
	}

	c.conn = nil
	c.logger.Info("WebSocket connection closed")
	return nil
}