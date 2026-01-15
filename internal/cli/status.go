package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
	"github.com/pfrederiksen/rivian-ls/internal/store"
)

// StatusOptions configures the status command
type StatusOptions struct {
	Format  OutputFormat
	Pretty  bool
	Offline bool // Use cached state instead of live query
}

// StatusCommand displays current vehicle state
type StatusCommand struct {
	client      rivian.Client
	store       *store.Store
	vehicleID   string
	vehicleName string
	vehicleVIN  string
	vehicleModel string
	output      io.Writer
}

// NewStatusCommand creates a new status command
func NewStatusCommand(client rivian.Client, store *store.Store, vehicleID string, output io.Writer) *StatusCommand {
	return &StatusCommand{
		client:    client,
		store:     store,
		vehicleID: vehicleID,
		output:    output,
	}
}

// SetVehicleInfo sets the vehicle identity information from GetVehicles.
// This enriches the state with name/model/VIN which aren't in GetVehicleState.
func (c *StatusCommand) SetVehicleInfo(name, vin, model string) {
	c.vehicleName = name
	c.vehicleVIN = vin
	c.vehicleModel = model
}

// Run executes the status command
func (c *StatusCommand) Run(ctx context.Context, opts StatusOptions) error {
	var state *model.VehicleState
	var err error

	if opts.Offline {
		// Use cached state from store
		state, err = c.store.GetLatestState(ctx, c.vehicleID)
		if err != nil {
			return fmt.Errorf("get cached state: %w", err)
		}
		if state == nil {
			return fmt.Errorf("no cached state found for vehicle %s", c.vehicleID)
		}
	} else {
		// Fetch live state from API
		rivState, err := c.client.GetVehicleState(ctx, c.vehicleID)
		if err != nil {
			return fmt.Errorf("get vehicle state: %w", err)
		}

		state = model.FromRivianVehicleState(rivState)

		// Enrich with vehicle identity (not in GetVehicleState API)
		if c.vehicleName != "" {
			state.Name = c.vehicleName
		}
		if c.vehicleVIN != "" {
			state.VIN = c.vehicleVIN
		}
		if c.vehicleModel != "" {
			state.Model = c.vehicleModel
		}

		// Update derived metrics
		state.UpdateReadyScore()

		// Save to store for offline use
		if c.store != nil {
			if err := c.store.SaveState(ctx, state); err != nil {
				// Non-fatal: log error but continue
				fmt.Fprintf(os.Stderr, "Warning: Failed to save state: %v\n", err)
			}
		}
	}

	// Format and output
	formatter, err := NewFormatter(opts.Format, opts.Pretty)
	if err != nil {
		return fmt.Errorf("create formatter: %w", err)
	}

	return formatter.FormatState(c.output, state)
}
