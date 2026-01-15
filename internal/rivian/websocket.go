package rivian

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// WebSocket endpoint for Rivian real-time updates
	WebSocketURL = "wss://rivian.com/api/gql/gateway/graphql"

	// Timeouts and intervals
	PingInterval    = 30 * time.Second
	PongTimeout     = 10 * time.Second
	ReconnectDelay  = 5 * time.Second
	MaxReconnects   = 10
	WriteTimeout    = 10 * time.Second
	ReadBufferSize  = 1024
	WriteBufferSize = 1024
)

// WebSocketMessage represents a GraphQL WebSocket message
type WebSocketMessage struct {
	ID      string                 `json:"id,omitempty"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// SubscriptionCallback is called when a subscription message is received
type SubscriptionCallback func(data map[string]interface{})

// WebSocketClient manages WebSocket connections for real-time updates
type WebSocketClient struct {
	mu             sync.RWMutex
	conn           *websocket.Conn
	credentials    *Credentials
	csrfToken      string
	appSessionID   string
	subscriptions  map[string]SubscriptionCallback // subscription ID -> callback
	reconnectCount int
	closeSignal    chan struct{}
	closed         bool
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(credentials *Credentials, csrfToken, appSessionID string) *WebSocketClient {
	return &WebSocketClient{
		credentials:   credentials,
		csrfToken:     csrfToken,
		appSessionID:  appSessionID,
		subscriptions: make(map[string]SubscriptionCallback),
		closeSignal:   make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection
func (c *WebSocketClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return fmt.Errorf("already connected")
	}

	// Set up WebSocket dialer with headers
	dialer := websocket.Dialer{
		ReadBufferSize:  ReadBufferSize,
		WriteBufferSize: WriteBufferSize,
	}

	headers := make(map[string][]string)
	headers["apollographql-client-name"] = []string{ApolloClientName}
	headers["Sec-WebSocket-Protocol"] = []string{"graphql-ws"}

	if c.appSessionID != "" {
		headers["a-sess"] = []string{c.appSessionID}
	}
	if c.csrfToken != "" {
		headers["csrf-token"] = []string{c.csrfToken}
	}
	if c.credentials != nil && c.credentials.AccessToken != "" {
		headers["u-sess"] = []string{c.credentials.AccessToken}
	}

	// Connect
	conn, _, err := dialer.DialContext(ctx, WebSocketURL, headers)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.conn = conn
	c.closed = false
	c.reconnectCount = 0

	// Send connection_init message
	initMsg := WebSocketMessage{
		Type: "connection_init",
		Payload: map[string]interface{}{
			"apollographql-client-name": ApolloClientName,
		},
	}

	if err := c.writeMessage(initMsg); err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("send connection_init: %w", err)
	}

	// Start message handler
	go c.messageLoop()

	// Start ping handler
	go c.pingLoop()

	return nil
}

// Subscribe subscribes to a GraphQL subscription
func (c *WebSocketClient) Subscribe(ctx context.Context, id, query string, variables map[string]interface{}, callback SubscriptionCallback) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Store callback
	c.subscriptions[id] = callback

	// Send start message
	msg := WebSocketMessage{
		ID:   id,
		Type: "start",
		Payload: map[string]interface{}{
			"query":     query,
			"variables": variables,
		},
	}

	if err := c.writeMessage(msg); err != nil {
		delete(c.subscriptions, id)
		return fmt.Errorf("send start: %w", err)
	}

	return nil
}

// Unsubscribe stops a subscription
func (c *WebSocketClient) Unsubscribe(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Remove callback
	delete(c.subscriptions, id)

	// Send stop message
	msg := WebSocketMessage{
		ID:   id,
		Type: "stop",
	}

	return c.writeMessage(msg)
}

// Close closes the WebSocket connection
func (c *WebSocketClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.closeSignal)

	if c.conn != nil {
		// Send connection_terminate
		msg := WebSocketMessage{
			Type: "connection_terminate",
		}
		_ = c.writeMessage(msg) // Best effort

		err := c.conn.Close()
		c.conn = nil
		return err
	}

	return nil
}

// messageLoop handles incoming WebSocket messages
func (c *WebSocketClient) messageLoop() {
	for {
		select {
		case <-c.closeSignal:
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		var msg WebSocketMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Unexpected close, attempt reconnect
				c.handleDisconnect()
			}
			return
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes a WebSocket message
func (c *WebSocketClient) handleMessage(msg WebSocketMessage) {
	switch msg.Type {
	case "connection_ack":
		// Connection acknowledged, ready to subscribe
		return

	case "ka": // Keep-alive
		return

	case "data":
		// Subscription data
		c.mu.RLock()
		callback, ok := c.subscriptions[msg.ID]
		c.mu.RUnlock()

		if ok && callback != nil {
			callback(msg.Payload)
		}

	case "error":
		// Subscription error
		// TODO: Log error from msg.Payload

	case "complete":
		// Subscription completed
		c.mu.Lock()
		delete(c.subscriptions, msg.ID)
		c.mu.Unlock()
	}
}

// handleDisconnect attempts to reconnect
func (c *WebSocketClient) handleDisconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	// Don't reconnect if we've exceeded max attempts
	if c.reconnectCount >= MaxReconnects {
		c.closed = true
		close(c.closeSignal)
		return
	}

	c.reconnectCount++

	// Wait before reconnecting
	time.Sleep(ReconnectDelay)

	// Attempt reconnect
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		// Reconnect failed, will try again on next disconnect
		return
	}

	// Resubscribe to all subscriptions
	for id, callback := range c.subscriptions {
		// Note: We'd need to store the original query/variables to resubscribe
		// For now, this is a simplified implementation
		_ = id
		_ = callback
	}
}

// pingLoop sends periodic pings to keep connection alive
func (c *WebSocketClient) pingLoop() {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeSignal:
			return
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				return
			}

			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(WriteTimeout)); err != nil {
				// Ping failed, connection might be dead
				c.handleDisconnect()
				return
			}
		}
	}
}

// writeMessage writes a message to the WebSocket
func (c *WebSocketClient) writeMessage(msg WebSocketMessage) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	c.conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	return c.conn.WriteJSON(msg)
}

// VehicleStateSubscription represents a subscription to vehicle state updates
type VehicleStateSubscription struct {
	client     *WebSocketClient
	vehicleID  string
	updateChan chan map[string]interface{}
}

// SubscribeToVehicleState creates a subscription for vehicle state updates
func SubscribeToVehicleState(ctx context.Context, client *WebSocketClient, vehicleID string) (*VehicleStateSubscription, error) {
	subscription := &VehicleStateSubscription{
		client:     client,
		vehicleID:  vehicleID,
		updateChan: make(chan map[string]interface{}, 10),
	}

	// GraphQL subscription query for vehicle state changes
	query := `
		subscription VehicleStateUpdates($vehicleId: String!) {
			vehicleState(id: $vehicleId) {
				__typename
				batteryLevel { value timeStamp }
				chargeState { value timeStamp }
				rangeEstimate { value timeStamp }
				isLocked { value timeStamp }
				cabinTemp { value timeStamp }
			}
		}
	`

	variables := map[string]interface{}{
		"vehicleId": vehicleID,
	}

	callback := func(data map[string]interface{}) {
		select {
		case subscription.updateChan <- data:
		default:
			// Channel full, drop update
		}
	}

	err := client.Subscribe(ctx, fmt.Sprintf("vehicle-state-%s", vehicleID), query, variables, callback)
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

// Updates returns the channel for receiving state updates
func (s *VehicleStateSubscription) Updates() <-chan map[string]interface{} {
	return s.updateChan
}

// Close closes the subscription
func (s *VehicleStateSubscription) Close() error {
	if err := s.client.Unsubscribe(fmt.Sprintf("vehicle-state-%s", s.vehicleID)); err != nil {
		return err
	}
	close(s.updateChan)
	return nil
}
