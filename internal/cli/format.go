package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/model"
	"gopkg.in/yaml.v3"
)

// OutputFormat represents supported output formats
type OutputFormat string

const (
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
	FormatCSV   OutputFormat = "csv"
	FormatTable OutputFormat = "table"
	FormatText  OutputFormat = "text"
)

// Formatter handles output formatting
type Formatter interface {
	FormatState(w io.Writer, state *model.VehicleState) error
	FormatStates(w io.Writer, states []*model.VehicleState) error
}

// JSONFormatter formats output as JSON
type JSONFormatter struct {
	Pretty bool
}

func (f *JSONFormatter) FormatState(w io.Writer, state *model.VehicleState) error {
	encoder := json.NewEncoder(w)
	if f.Pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(state)
}

func (f *JSONFormatter) FormatStates(w io.Writer, states []*model.VehicleState) error {
	encoder := json.NewEncoder(w)
	if f.Pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(states)
}

// YAMLFormatter formats output as YAML
type YAMLFormatter struct{}

func (f *YAMLFormatter) FormatState(w io.Writer, state *model.VehicleState) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(state)
}

func (f *YAMLFormatter) FormatStates(w io.Writer, states []*model.VehicleState) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(states)
}

// CSVFormatter formats output as CSV
type CSVFormatter struct{}

func (f *CSVFormatter) FormatState(w io.Writer, state *model.VehicleState) error {
	return f.FormatStates(w, []*model.VehicleState{state})
}

func (f *CSVFormatter) FormatStates(w io.Writer, states []*model.VehicleState) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{
		"Timestamp", "VehicleID", "VIN", "Name", "Model",
		"BatteryLevel", "RangeEstimate", "RangeStatus",
		"ChargeState", "ChargeLimit", "ChargingRate",
		"IsLocked", "IsOnline",
		"Latitude", "Longitude",
		"CabinTemp", "ExteriorTemp",
		"Odometer",
		"ReadyScore",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write rows
	for _, state := range states {
		row := []string{
			state.UpdatedAt.Format(time.RFC3339),
			state.VehicleID,
			state.VIN,
			state.Name,
			state.Model,
			formatFloat(state.BatteryLevel, 1),
			formatFloat(state.RangeEstimate, 1),
			string(state.RangeStatus),
			string(state.ChargeState),
			strconv.Itoa(state.ChargeLimit),
			formatFloatPtr(state.ChargingRate, 1),
			formatBool(state.IsLocked),
			formatBool(state.IsOnline),
			formatLocation(state.Location, true),
			formatLocation(state.Location, false),
			formatFloatPtr(state.CabinTemp, 1),
			formatFloatPtr(state.ExteriorTemp, 1),
			formatFloat(state.Odometer, 1),
			formatFloatPtr(state.ReadyScore, 1),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// TextFormatter formats output as human-readable text
type TextFormatter struct{}

func (f *TextFormatter) FormatState(w io.Writer, state *model.VehicleState) error {
	fmt.Fprintf(w, "Vehicle: %s (%s)\n", state.Name, state.Model)
	fmt.Fprintf(w, "VIN: %s\n", state.VIN)
	fmt.Fprintf(w, "Status: %s\n", formatOnlineStatus(state.IsOnline))
	fmt.Fprintf(w, "Updated: %s\n", state.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "\n")

	// Battery & Range
	fmt.Fprintf(w, "Battery: %.1f%% | Range: %.0f miles (%s)\n",
		state.BatteryLevel, state.RangeEstimate, state.RangeStatus)

	// Charging
	if state.ChargeState == model.ChargeStateCharging {
		rate := ""
		if state.ChargingRate != nil {
			rate = fmt.Sprintf(" @ %.1f kW", *state.ChargingRate)
		}
		timeToCharge := ""
		if state.TimeToCharge != nil {
			remaining := state.TimeToCharge.Sub(state.UpdatedAt)
			timeToCharge = fmt.Sprintf(" (%s remaining)", formatDuration(remaining))
		}
		fmt.Fprintf(w, "Charging: %s%s%s\n", state.ChargeState, rate, timeToCharge)
	} else {
		fmt.Fprintf(w, "Charging: %s | Limit: %d%%\n", state.ChargeState, state.ChargeLimit)
	}
	fmt.Fprintf(w, "\n")

	// Security & Closures
	fmt.Fprintf(w, "Lock: %s\n", formatLockStatus(state.IsLocked))
	fmt.Fprintf(w, "Doors: %s | Windows: %s\n",
		formatClosures(state.Doors), formatClosures(state.Windows))

	// Only show closures if known
	closureLine := ""
	if state.Frunk != model.ClosureStatusUnknown {
		closureLine += fmt.Sprintf("Frunk: %s", state.Frunk)
	}
	if state.Liftgate != model.ClosureStatusUnknown {
		if closureLine != "" {
			closureLine += " | "
		}
		closureLine += fmt.Sprintf("Liftgate: %s", state.Liftgate)
	}
	if state.TonneauCover != nil && *state.TonneauCover != model.ClosureStatusUnknown {
		if closureLine != "" {
			closureLine += " | "
		}
		closureLine += fmt.Sprintf("Tonneau: %s", *state.TonneauCover)
	}
	if closureLine != "" {
		fmt.Fprintf(w, "%s\n", closureLine)
	}
	fmt.Fprintf(w, "\n")

	// Climate
	if state.CabinTemp != nil || state.ExteriorTemp != nil {
		fmt.Fprintf(w, "Temperature: ")
		if state.CabinTemp != nil {
			fmt.Fprintf(w, "Cabin %.1f°F", *state.CabinTemp)
		}
		if state.ExteriorTemp != nil {
			if state.CabinTemp != nil {
				fmt.Fprintf(w, " | ")
			}
			fmt.Fprintf(w, "Exterior %.1f°F", *state.ExteriorTemp)
		}
		fmt.Fprintf(w, "\n")
	}

	// Location
	if state.Location != nil {
		fmt.Fprintf(w, "Location: %.4f°N, %.4f°W\n",
			state.Location.Latitude, state.Location.Longitude)
	}

	// Odometer
	fmt.Fprintf(w, "Odometer: %.1f miles\n", state.Odometer)
	fmt.Fprintf(w, "\n")

	// Ready Score
	if state.ReadyScore != nil {
		fmt.Fprintf(w, "Ready Score: %.1f/100\n", *state.ReadyScore)
	}

	// Issues
	issues := state.GetIssues()
	if len(issues) > 0 {
		fmt.Fprintf(w, "\nIssues:\n")
		for _, issue := range issues {
			fmt.Fprintf(w, "  • %s\n", issue)
		}
	}

	return nil
}

func (f *TextFormatter) FormatStates(w io.Writer, states []*model.VehicleState) error {
	for i, state := range states {
		if i > 0 {
			fmt.Fprintf(w, "\n---\n\n")
		}
		if err := f.FormatState(w, state); err != nil {
			return err
		}
	}
	return nil
}

// TableFormatter formats output as a compact table
type TableFormatter struct{}

func (f *TableFormatter) FormatState(w io.Writer, state *model.VehicleState) error {
	return f.FormatStates(w, []*model.VehicleState{state})
}

func (f *TableFormatter) FormatStates(w io.Writer, states []*model.VehicleState) error {
	if len(states) == 0 {
		fmt.Fprintf(w, "No states to display\n")
		return nil
	}

	// Header
	fmt.Fprintf(w, "%-19s  %-8s  %-6s  %-5s  %-10s  %s\n",
		"TIMESTAMP", "BATTERY", "RANGE", "LOCK", "CHARGING", "STATUS")
	fmt.Fprintf(w, "%-19s  %-8s  %-6s  %-5s  %-10s  %s\n",
		"-------------------", "--------", "------", "-----", "----------", "------")

	// Rows
	for _, state := range states {
		fmt.Fprintf(w, "%-19s  %6.1f%%  %5.0fmi  %-5s  %-10s  %s\n",
			state.UpdatedAt.Format("2006-01-02 15:04:05"),
			state.BatteryLevel,
			state.RangeEstimate,
			formatLockStatusShort(state.IsLocked),
			state.ChargeState,
			formatOnlineStatusShort(state.IsOnline),
		)
	}

	return nil
}

// NewFormatter creates a formatter for the given format
func NewFormatter(format OutputFormat, pretty bool) (Formatter, error) {
	switch format {
	case FormatJSON:
		return &JSONFormatter{Pretty: pretty}, nil
	case FormatYAML:
		return &YAMLFormatter{}, nil
	case FormatCSV:
		return &CSVFormatter{}, nil
	case FormatText:
		return &TextFormatter{}, nil
	case FormatTable:
		return &TableFormatter{}, nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

// Helper functions

func formatFloat(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}

func formatFloatPtr(f *float64, prec int) string {
	if f == nil {
		return ""
	}
	return formatFloat(*f, prec)
}

func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func formatLocation(loc *model.Location, latitude bool) string {
	if loc == nil {
		return ""
	}
	if latitude {
		return formatFloat(loc.Latitude, 4)
	}
	return formatFloat(loc.Longitude, 4)
}

func formatOnlineStatus(online bool) string {
	if online {
		return "Online"
	}
	return "Offline"
}

func formatOnlineStatusShort(online bool) string {
	if online {
		return "online"
	}
	return "offline"
}

func formatLockStatus(locked bool) string {
	if locked {
		return "Locked"
	}
	return "Unlocked"
}

func formatLockStatusShort(locked bool) string {
	if locked {
		return "🔒"
	}
	return "🔓"
}

func formatClosures(closures model.Closures) string {
	if closures.AllClosed() {
		return "All closed"
	}
	if closures.AnyOpen() {
		count := 0
		if closures.FrontLeft == model.ClosureStatusOpen {
			count++
		}
		if closures.FrontRight == model.ClosureStatusOpen {
			count++
		}
		if closures.RearLeft == model.ClosureStatusOpen {
			count++
		}
		if closures.RearRight == model.ClosureStatusOpen {
			count++
		}
		return fmt.Sprintf("%d open", count)
	}
	return "Unknown"
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "0m"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
