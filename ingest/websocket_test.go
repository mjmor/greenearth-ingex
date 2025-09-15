package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// MockWebSocketClient implements WebSocketClient for testing
type MockWebSocketClient struct {
	connectError    error
	readMessages    [][]byte
	readErrors      []error
	readIndex       int
	closeError      error
	connected       bool
	readMessageFunc func(ctx context.Context) ([]byte, error)
}

func NewMockWebSocketClient() *MockWebSocketClient {
	return &MockWebSocketClient{
		readMessages: make([][]byte, 0),
		readErrors:   make([]error, 0),
	}
}

func (m *MockWebSocketClient) Connect(ctx context.Context, url string) error {
	if m.connectError != nil {
		return m.connectError
	}
	m.connected = true
	return nil
}

func (m *MockWebSocketClient) ReadMessage(ctx context.Context) ([]byte, error) {
	if m.readMessageFunc != nil {
		return m.readMessageFunc(ctx)
	}

	if m.readIndex >= len(m.readMessages) {
		if m.readIndex < len(m.readErrors) {
			err := m.readErrors[m.readIndex]
			m.readIndex++
			return nil, err
		}
		return nil, context.DeadlineExceeded
	}

	message := m.readMessages[m.readIndex]
	m.readIndex++
	return message, nil
}

func (m *MockWebSocketClient) Close() error {
	m.connected = false
	return m.closeError
}

func (m *MockWebSocketClient) SetMessages(messages [][]byte) {
	m.readMessages = messages
	m.readIndex = 0
}

func (m *MockWebSocketClient) SetErrors(errors []error) {
	m.readErrors = errors
}

func TestNewTurboStreamClient(t *testing.T) {
	logger := NewLogger(false)
	client := NewTurboStreamClient(logger)

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.logger != logger {
		t.Error("Expected logger to be set")
	}
}

func TestTurboStreamClient_ConnectSuccess(t *testing.T) {
	// Create a test WebSocket server
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
		}
		defer conn.Close()

		// Keep connection open for a bit
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	logger := NewLogger(false)
	client := NewTurboStreamClient(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx, wsURL)
	if err != nil {
		t.Fatalf("Expected successful connection, got error: %v", err)
	}

	// Clean up
	client.Close()
}

func TestTurboStreamClient_ConnectFailure(t *testing.T) {
	logger := NewLogger(false)
	client := NewTurboStreamClient(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Try to connect to invalid URL
	err := client.Connect(ctx, "ws://invalid-url:12345")
	if err == nil {
		t.Fatal("Expected connection to fail, but it succeeded")
	}
}

func TestTurboStreamClient_ReadMessageWithoutConnection(t *testing.T) {
	logger := NewLogger(false)
	client := NewTurboStreamClient(logger)

	ctx := context.Background()
	_, err := client.ReadMessage(ctx)
	if err == nil {
		t.Fatal("Expected error when reading without connection")
	}
}

func TestTurboStreamClient_Close(t *testing.T) {
	logger := NewLogger(false)
	client := NewTurboStreamClient(logger)

	// Should not error when closing without connection
	err := client.Close()
	if err != nil {
		t.Fatalf("Expected no error when closing without connection, got: %v", err)
	}
}

func TestMockWebSocketClient(t *testing.T) {
	mock := NewMockWebSocketClient()

	// Test Connect
	ctx := context.Background()
	err := mock.Connect(ctx, "ws://test")
	if err != nil {
		t.Fatalf("Expected successful mock connection, got: %v", err)
	}

	if !mock.connected {
		t.Error("Expected mock to be connected")
	}

	// Test ReadMessage with preset messages
	testMessages := [][]byte{
		[]byte(`{"type": "test1"}`),
		[]byte(`{"type": "test2"}`),
	}
	mock.SetMessages(testMessages)

	message1, err := mock.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("Expected successful read, got: %v", err)
	}

	if string(message1) != string(testMessages[0]) {
		t.Errorf("Expected first message, got: %s", string(message1))
	}

	message2, err := mock.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("Expected successful read, got: %v", err)
	}

	if string(message2) != string(testMessages[1]) {
		t.Errorf("Expected second message, got: %s", string(message2))
	}

	// Test Close
	err = mock.Close()
	if err != nil {
		t.Fatalf("Expected successful close, got: %v", err)
	}

	if mock.connected {
		t.Error("Expected mock to be disconnected")
	}
}