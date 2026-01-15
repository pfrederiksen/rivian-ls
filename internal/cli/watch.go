package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
	"github.com/pfrederiksen/rivian-ls/internal/store"
)

// WatchOptions configures the watch command
type WatchOptions struct {
	Format   OutputFormat
	Pretty   bool
	Interval time.Duration // Polling interval (0 = use WebSocket)
}

// WatchCommand streams real-time vehicle state updates
type WatchCommand struct {
	client    rivian.Client
	store     *store.Store
	vehicleID string
	csrfToken string
	appSessID string
	output    io.Writer
}

// NewWatchCommand creates a new watch command
func NewWatchCommand(client rivian.Client, store *store.Store, vehicleID, csrfToken, appSessID string, output io.Writer) *WatchCommand {
	return &WatchCommand{
		client:    client,
		store:     store,
		vehicleID: vehicleID,
		csrfToken: csrfToken,
		appSessID: appSessID,
		output:    output,
	}
}

// Run executes the watch command
func (c *WatchCommand) Run(ctx context.Context, opts WatchOptions) error {
	formatter, err := NewFormatter(opts.Format, opts.Pretty)
	if err != nil {
		return fmt.Errorf("create formatter: %w", err)
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		_, _ = fmt.Fprintln(os.Stderr, "\nShutting down...")
		cancel()
	}()

	if opts.Interval > 0 {
		// Polling mode
		return c.runPolling(ctx, formatter, opts.Interval)
	}

	// WebSocket mode with fallback to polling
	wsErr := c.runWebSocket(ctx, formatter)
	if wsErr != nil {
		// WebSocket failed - fall back to polling mode
		_, _ = fmt.Fprintln(os.Stderr, "\nWebSocket connection failed (this is a known Rivian API limitation).")
		_, _ = fmt.Fprintln(os.Stderr, "Falling back to polling mode (30-second intervals)...")
		return c.runPolling(ctx, formatter, 30*time.Second)
	}
	return nil
}

// runPolling implements polling-based updates
func (c *WatchCommand) runPolling(ctx context.Context, formatter Formatter, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Get initial state
	if err := c.fetchAndOutput(ctx, formatter); err != nil {
		return err
	}

	// Poll for updates
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := c.fetchAndOutput(ctx, formatter); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error fetching state: %v\n", err)
				// Continue polling despite errors
			}
		}
	}
}

// runWebSocket implements WebSocket-based updates
func (c *WatchCommand) runWebSocket(ctx context.Context, formatter Formatter) error {
	// Get credentials from HTTP client
	httpClient, ok := c.client.(*rivian.HTTPClient)
	if !ok {
		return fmt.Errorf("WebSocket mode requires HTTPClient")
	}

	creds := httpClient.GetCredentials()
	if creds == nil {
		return fmt.Errorf("not authenticated")
	}

	// Create WebSocket client
	wsClient := rivian.NewWebSocketClient(creds, c.csrfToken, c.appSessID)

	// Connect
	if err := wsClient.Connect(ctx); err != nil {
		return fmt.Errorf("connect websocket: %w", err)
	}
	defer func() { _ = wsClient.Close() }()

	// Subscribe to vehicle state updates
	subscription, err := rivian.SubscribeToVehicleState(ctx, wsClient, c.vehicleID)
	if err != nil {
		return fmt.Errorf("subscribe to vehicle state: %w", err)
	}
	defer func() { _ = subscription.Close() }()

	_, _ = fmt.Fprintln(os.Stderr, "Watching for updates... (Press Ctrl+C to stop)")

	// Get initial state via HTTP
	if err := c.fetchAndOutput(ctx, formatter); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to get initial state: %v\n", err)
	}

	// Process updates
	reducer := model.NewReducer()

	for {
		select {
		case <-ctx.Done():
			return nil

		case update := <-subscription.Updates():
			if update == nil {
				continue
			}

			// Convert WebSocket update to partial state update
			updates := extractVehicleStateUpdates(update)
			if len(updates) == 0 {
				continue
			}

			// Apply partial update
			event := model.PartialStateUpdate{
				VehicleID: c.vehicleID,
				Updates:   updates,
			}
			state := reducer.Dispatch(event)

			// Output updated state
			if err := formatter.FormatState(c.output, state); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error formatting state: %v\n", err)
			}

			// Save to store
			if c.store != nil {
				if err := c.store.SaveState(ctx, state); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to save state: %v\n", err)
				}
			}
		}
	}
}

// extractVehicleStateUpdates parses WebSocket update payload into field updates
func extractVehicleStateUpdates(update map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})

	data, ok := update["data"].(map[string]interface{})
	if !ok {
		return updates
	}

	vState, ok := data["vehicleState"].(map[string]interface{})
	if !ok {
		return updates
	}

	// Extract timestamped values using helper
	extractTimestampedFloat(vState, "batteryLevel", "batteryLevel", updates)
	extractTimestampedFloat(vState, "rangeEstimate", "rangeEstimate", updates)
	extractTimestampedString(vState, "chargeState", "chargeState", updates)
	extractTimestampedBool(vState, "isLocked", "isLocked", updates)
	extractTimestampedFloat(vState, "cabinTemp", "cabinTemp", updates)

	return updates
}

// extractTimestampedFloat extracts a float value from a timestamped field
func extractTimestampedFloat(vState map[string]interface{}, field, key string, updates map[string]interface{}) {
	if fieldData, ok := vState[field].(map[string]interface{}); ok {
		if val, ok := fieldData["value"].(float64); ok {
			updates[key] = val
		}
	}
}

// extractTimestampedString extracts a string value from a timestamped field
func extractTimestampedString(vState map[string]interface{}, field, key string, updates map[string]interface{}) {
	if fieldData, ok := vState[field].(map[string]interface{}); ok {
		if val, ok := fieldData["value"].(string); ok {
			updates[key] = val
		}
	}
}

// extractTimestampedBool extracts a bool value from a timestamped field
func extractTimestampedBool(vState map[string]interface{}, field, key string, updates map[string]interface{}) {
	if fieldData, ok := vState[field].(map[string]interface{}); ok {
		if val, ok := fieldData["value"].(bool); ok {
			updates[key] = val
		}
	}
}

// fetchAndOutput fetches current state and outputs it
func (c *WatchCommand) fetchAndOutput(ctx context.Context, formatter Formatter) error {
	rivState, err := c.client.GetVehicleState(ctx, c.vehicleID)
	if err != nil {
		return err
	}

	state := model.FromRivianVehicleState(rivState)
	state.UpdateReadyScore()

	// Save to store
	if c.store != nil {
		if err := c.store.SaveState(ctx, state); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save state: %v\n", err)
		}
	}

	return formatter.FormatState(c.output, state)
}
