package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/store"
)

// ExportOptions configures the export command
type ExportOptions struct {
	Format OutputFormat
	Pretty bool
	Since  time.Time // Start time for export
	Until  time.Time // End time for export
	Limit  int       // Maximum number of records
}

// ExportCommand exports historical vehicle state data
type ExportCommand struct {
	store     *store.Store
	vehicleID string
	output    io.Writer
}

// NewExportCommand creates a new export command
func NewExportCommand(store *store.Store, vehicleID string, output io.Writer) *ExportCommand {
	return &ExportCommand{
		store:     store,
		vehicleID: vehicleID,
		output:    output,
	}
}

// Run executes the export command
func (c *ExportCommand) Run(ctx context.Context, opts ExportOptions) error {
	if c.store == nil {
		return fmt.Errorf("store not available for export")
	}

	var states []*model.VehicleState
	var err error

	// Determine query method
	switch {
	case !opts.Since.IsZero() && !opts.Until.IsZero():
		// Range query
		states, err = c.store.GetStates(ctx, c.vehicleID, opts.Since, opts.Until)
	case !opts.Since.IsZero():
		// History query with limit
		limit := opts.Limit
		if limit == 0 {
			limit = 1000 // Default limit
		}
		states, err = c.store.GetStateHistory(ctx, c.vehicleID, opts.Since, limit)
	default:
		// Get all recent states
		limit := opts.Limit
		if limit == 0 {
			limit = 100 // Default limit for unbounded query
		}
		since := time.Now().AddDate(-1, 0, 0) // Last year
		states, err = c.store.GetStateHistory(ctx, c.vehicleID, since, limit)
	}

	if err != nil {
		return fmt.Errorf("query states: %w", err)
	}

	if len(states) == 0 {
		_, _ = fmt.Fprintln(c.output, "No states found for the specified time range")
		return nil
	}

	// Format and output
	formatter, err := NewFormatter(opts.Format, opts.Pretty)
	if err != nil {
		return fmt.Errorf("create formatter: %w", err)
	}

	return formatter.FormatStates(c.output, states)
}
