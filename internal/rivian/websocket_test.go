package rivian

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// mockWebSocketServer creates a test WebSocket server
type mockWebSocketServer struct {
	server   *httptest.Server
	upgrader websocket.Upgrader
	messages chan WebSocketMessage
	clients  []*websocket.Conn
}

func newMockWebSocketServer() *mockWebSocketServer {
	mock := &mockWebSocketServer{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		messages: make(chan WebSocketMessage, 10),
		clients:  make([]*websocket.Conn, 0),
	}

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := mock.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mock.clients = append(mock.clients, conn)
		mock.handleConnection(conn)
	}))

	return mock
}

func (m *mockWebSocketServer) handleConnection(conn *websocket.Conn) {
	defer conn.Close()

	for {
		var msg WebSocketMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		m.messages <- msg

		// Handle different message types
		switch msg.Type {
		case "connection_init":
			// Send connection_ack
			ack := WebSocketMessage{Type: "connection_ack"}
			if err := conn.WriteJSON(ack); err != nil {
				return
			}

		case "start":
			// Echo subscription started (in real API, this would send data updates)
			// For testing, we'll send a mock data message after a short delay
			go func() {
				time.Sleep(50 * time.Millisecond)
				data := WebSocketMessage{
					ID:   msg.ID,
					Type: "data",
					Payload: map[string]interface{}{
						"data": map[string]interface{}{
							"vehicleState": map[string]interface{}{
								"batteryLevel": map[string]interface{}{
									"value":     85.5,
									"timeStamp": "2024-01-15T10:00:00Z",
								},
							},
						},
					},
				}
				conn.WriteJSON(data)
			}()

		case "stop":
			// Send complete
			complete := WebSocketMessage{
				ID:   msg.ID,
				Type: "complete",
			}
			if err := conn.WriteJSON(complete); err != nil {
				return
			}

		case "connection_terminate":
			return
		}
	}
}

func (m *mockWebSocketServer) close() {
	for _, conn := range m.clients {
		conn.Close()
	}
	m.server.Close()
}

func (m *mockWebSocketServer) url() string {
	return "ws" + strings.TrimPrefix(m.server.URL, "http")
}

func TestNewWebSocketClient(t *testing.T) {
	creds := &Credentials{AccessToken: "test-token"}
	client := NewWebSocketClient(creds, "csrf-123", "app-session-123")

	if client.credentials != creds {
		t.Error("credentials not set")
	}
	if client.csrfToken != "csrf-123" {
		t.Error("csrfToken not set")
	}
	if client.appSessionID != "app-session-123" {
		t.Error("appSessionID not set")
	}
	if client.subscriptions == nil {
		t.Error("subscriptions map not initialized")
	}
}

func TestWebSocketClient_Connect(t *testing.T) {
	mock := newMockWebSocketServer()
	defer mock.close()

	// Override WebSocketURL for testing
	originalURL := WebSocketURL
	defer func() {
		// Note: We can't actually override the const, so this test uses the mock server URL directly
	}()
	_ = originalURL

	creds := &Credentials{AccessToken: "test-token"}
	client := NewWebSocketClient(creds, "csrf-123", "app-session-123")

	// Manually set connection for testing (since we can't override const URL)
	ctx := context.Background()

	// Create a custom client that connects to our mock server
	dialer := websocket.Dialer{
		ReadBufferSize:  ReadBufferSize,
		WriteBufferSize: WriteBufferSize,
	}

	headers := make(map[string][]string)
	headers["apollographql-client-name"] = []string{ApolloClientName}
	headers["Sec-WebSocket-Protocol"] = []string{"graphql-ws"}
	headers["a-sess"] = []string{client.appSessionID}
	headers["csrf-token"] = []string{client.csrfToken}
	headers["u-sess"] = []string{client.credentials.AccessToken}

	conn, _, err := dialer.DialContext(ctx, mock.url(), headers)
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}

	// Manually set up the client
	client.mu.Lock()
	client.conn = conn
	client.closed = false
	client.mu.Unlock()

	// Send connection_init
	initMsg := WebSocketMessage{
		Type: "connection_init",
		Payload: map[string]interface{}{
			"apollographql-client-name": ApolloClientName,
		},
	}

	if err := client.writeMessage(initMsg); err != nil {
		t.Fatalf("Failed to send connection_init: %v", err)
	}

	// Wait for connection_init to be received by server
	select {
	case msg := <-mock.messages:
		if msg.Type != "connection_init" {
			t.Errorf("Expected connection_init, got %s", msg.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for connection_init")
	}

	// Clean up
	client.Close()
}

func TestWebSocketClient_Subscribe(t *testing.T) {
	mock := newMockWebSocketServer()
	defer mock.close()

	creds := &Credentials{AccessToken: "test-token"}
	client := NewWebSocketClient(creds, "csrf-123", "app-session-123")

	// Set up connection to mock server
	ctx := context.Background()
	dialer := websocket.Dialer{
		ReadBufferSize:  ReadBufferSize,
		WriteBufferSize: WriteBufferSize,
	}

	conn, _, err := dialer.DialContext(ctx, mock.url(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	client.mu.Lock()
	client.conn = conn
	client.closed = false
	client.closeSignal = make(chan struct{})
	client.mu.Unlock()

	// Start message loop
	go client.messageLoop()

	// Subscribe
	callbackCalled := false
	callback := func(data map[string]interface{}) {
		callbackCalled = true
	}

	query := "subscription { test }"
	variables := map[string]interface{}{"id": "123"}

	err = client.Subscribe(ctx, "sub-1", query, variables, callback)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Wait for start message
	select {
	case msg := <-mock.messages:
		if msg.Type != "start" {
			t.Errorf("Expected start, got %s", msg.Type)
		}
		if msg.ID != "sub-1" {
			t.Errorf("Expected ID sub-1, got %s", msg.ID)
		}

		// Verify payload
		if msg.Payload["query"] != query {
			t.Errorf("Query mismatch")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for start message")
	}

	// Verify subscription was stored
	client.mu.RLock()
	_, ok := client.subscriptions["sub-1"]
	client.mu.RUnlock()

	if !ok {
		t.Error("Subscription not stored")
	}

	// Wait for mock data message
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Callback was not called")
	}

	// Clean up
	client.Close()
}

func TestWebSocketClient_Unsubscribe(t *testing.T) {
	mock := newMockWebSocketServer()
	defer mock.close()

	creds := &Credentials{AccessToken: "test-token"}
	client := NewWebSocketClient(creds, "csrf-123", "app-session-123")

	// Set up connection
	ctx := context.Background()
	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, mock.url(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	client.mu.Lock()
	client.conn = conn
	client.subscriptions["sub-1"] = func(data map[string]interface{}) {}
	client.mu.Unlock()

	// Unsubscribe
	err = client.Unsubscribe("sub-1")
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Verify subscription was removed
	client.mu.RLock()
	_, ok := client.subscriptions["sub-1"]
	client.mu.RUnlock()

	if ok {
		t.Error("Subscription not removed")
	}

	// Verify stop message was sent (skip connection_init)
	timeout := time.After(1 * time.Second)
	for {
		select {
		case msg := <-mock.messages:
			if msg.Type == "stop" {
				if msg.ID != "sub-1" {
					t.Errorf("Expected ID sub-1, got %s", msg.ID)
				}
				goto done
			}
			// Skip other messages like connection_init
		case <-timeout:
			t.Fatal("Timeout waiting for stop message")
		}
	}
done:

	// Clean up
	client.Close()
}

func TestWebSocketClient_Close(t *testing.T) {
	mock := newMockWebSocketServer()
	defer mock.close()

	creds := &Credentials{AccessToken: "test-token"}
	client := NewWebSocketClient(creds, "csrf-123", "app-session-123")

	// Set up connection
	ctx := context.Background()
	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, mock.url(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	client.mu.Lock()
	client.conn = conn
	client.closeSignal = make(chan struct{})
	client.mu.Unlock()

	// Close
	err = client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify state
	client.mu.RLock()
	if !client.closed {
		t.Error("Client not marked as closed")
	}
	if client.conn != nil {
		t.Error("Connection not cleared")
	}
	client.mu.RUnlock()

	// Verify close is idempotent
	err = client.Close()
	if err != nil {
		t.Errorf("Second close should not error: %v", err)
	}
}

func TestWebSocketMessage_JSON(t *testing.T) {
	msg := WebSocketMessage{
		ID:   "test-id",
		Type: "start",
		Payload: map[string]interface{}{
			"query":     "subscription { test }",
			"variables": map[string]interface{}{"id": "123"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded WebSocketMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != msg.ID {
		t.Errorf("ID mismatch: got %s, want %s", decoded.ID, msg.ID)
	}
	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: got %s, want %s", decoded.Type, msg.Type)
	}
}

func TestWebSocketClient_HandleMessageTypes(t *testing.T) {
	creds := &Credentials{AccessToken: "test-token"}
	client := NewWebSocketClient(creds, "csrf-123", "app-session-123")

	tests := []struct {
		name    string
		message WebSocketMessage
		wantErr bool
	}{
		{
			name: "connection_ack",
			message: WebSocketMessage{
				Type: "connection_ack",
			},
		},
		{
			name: "keep-alive",
			message: WebSocketMessage{
				Type: "ka",
			},
		},
		{
			name: "error",
			message: WebSocketMessage{
				ID:   "sub-1",
				Type: "error",
				Payload: map[string]interface{}{
					"message": "subscription error",
				},
			},
		},
		{
			name: "complete",
			message: WebSocketMessage{
				ID:   "sub-1",
				Type: "complete",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up subscription for complete/error tests
			if tt.message.ID != "" {
				client.mu.Lock()
				client.subscriptions[tt.message.ID] = func(data map[string]interface{}) {}
				client.mu.Unlock()
			}

			// Handle message
			client.handleMessage(tt.message)

			// Verify complete removes subscription
			if tt.message.Type == "complete" {
				client.mu.RLock()
				_, ok := client.subscriptions[tt.message.ID]
				client.mu.RUnlock()

				if ok {
					t.Error("Subscription not removed after complete")
				}
			}
		})
	}
}

func TestVehicleStateSubscription(t *testing.T) {
	mock := newMockWebSocketServer()
	defer mock.close()

	creds := &Credentials{AccessToken: "test-token"}
	wsClient := NewWebSocketClient(creds, "csrf-123", "app-session-123")

	// Set up connection
	ctx := context.Background()
	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, mock.url(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	wsClient.mu.Lock()
	wsClient.conn = conn
	wsClient.closed = false
	wsClient.closeSignal = make(chan struct{})
	wsClient.mu.Unlock()

	// Start message loop
	go wsClient.messageLoop()

	// Create vehicle state subscription
	subscription, err := SubscribeToVehicleState(ctx, wsClient, "vehicle-123")
	if err != nil {
		t.Fatalf("SubscribeToVehicleState failed: %v", err)
	}

	// Wait for data update from mock
	select {
	case update := <-subscription.Updates():
		if update == nil {
			t.Fatal("Received nil update")
		}
		// Verify structure
		data, ok := update["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Invalid update structure")
		}
		state, ok := data["vehicleState"].(map[string]interface{})
		if !ok {
			t.Fatal("Invalid vehicleState structure")
		}
		battery, ok := state["batteryLevel"].(map[string]interface{})
		if !ok {
			t.Fatal("Invalid batteryLevel structure")
		}
		if battery["value"] != 85.5 {
			t.Errorf("Expected battery value 85.5, got %v", battery["value"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for update")
	}

	// Close subscription
	if err := subscription.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Clean up
	wsClient.Close()
}
