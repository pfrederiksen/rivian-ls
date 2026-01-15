package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/model"
	"gopkg.in/yaml.v3"
)

func makeTestState() *model.VehicleState {
	cabinTemp := 72.0
	chargingRate := 11.5
	readyScore := 95.0
	tonneauCover := model.ClosureStatusClosed

	return &model.VehicleState{
		VehicleID:       "vehicle-123",
		VIN:             "VIN123456",
		Name:            "My R1T",
		Model:           "R1T",
		UpdatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		BatteryLevel:    85.5,
		BatteryCapacity: 135.0,
		RangeEstimate:   250.0,
		RangeStatus:     model.RangeStatusNormal,
		ChargeState:     model.ChargeStateCharging,
		ChargeLimit:     80,
		ChargingRate:    &chargingRate,
		IsLocked:        true,
		IsOnline:        true,
		Odometer:        12345.6,
		CabinTemp:       &cabinTemp,
		Location: &model.Location{
			Latitude:  37.7749,
			Longitude: -122.4194,
		},
		Doors: model.Closures{
			FrontLeft:  model.ClosureStatusClosed,
			FrontRight: model.ClosureStatusClosed,
			RearLeft:   model.ClosureStatusClosed,
			RearRight:  model.ClosureStatusClosed,
		},
		Windows: model.Closures{
			FrontLeft:  model.ClosureStatusClosed,
			FrontRight: model.ClosureStatusClosed,
			RearLeft:   model.ClosureStatusClosed,
			RearRight:  model.ClosureStatusClosed,
		},
		Frunk:        model.ClosureStatusClosed,
		Liftgate:     model.ClosureStatusClosed,
		TonneauCover: &tonneauCover,
		TirePressures: model.TirePressures{
			FrontLeft:  42.0,
			FrontRight: 41.5,
			RearLeft:   42.0,
			RearRight:  41.5,
			UpdatedAt:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		ReadyScore: &readyScore,
	}
}

func TestJSONFormatter_FormatState(t *testing.T) {
	state := makeTestState()
	formatter := &JSONFormatter{Pretty: true}

	var buf bytes.Buffer
	err := formatter.FormatState(&buf, state)
	if err != nil {
		t.Fatalf("FormatState failed: %v", err)
	}

	// Verify it's valid JSON
	var decoded model.VehicleState
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Verify key fields
	if decoded.VehicleID != "vehicle-123" {
		t.Errorf("VehicleID mismatch: got %s", decoded.VehicleID)
	}
	if decoded.BatteryLevel != 85.5 {
		t.Errorf("BatteryLevel mismatch: got %v", decoded.BatteryLevel)
	}
}

func TestJSONFormatter_FormatStates(t *testing.T) {
	states := []*model.VehicleState{makeTestState(), makeTestState()}
	formatter := &JSONFormatter{Pretty: false}

	var buf bytes.Buffer
	err := formatter.FormatStates(&buf, states)
	if err != nil {
		t.Fatalf("FormatStates failed: %v", err)
	}

	// Verify it's valid JSON array
	var decoded []*model.VehicleState
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if len(decoded) != 2 {
		t.Errorf("Expected 2 states, got %d", len(decoded))
	}
}

func TestYAMLFormatter_FormatState(t *testing.T) {
	state := makeTestState()
	formatter := &YAMLFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatState(&buf, state)
	if err != nil {
		t.Fatalf("FormatState failed: %v", err)
	}

	// Verify it's valid YAML
	var decoded model.VehicleState
	if err := yaml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Invalid YAML: %v", err)
	}

	// Verify key fields
	if decoded.VehicleID != "vehicle-123" {
		t.Errorf("VehicleID mismatch: got %s", decoded.VehicleID)
	}
}

func TestYAMLFormatter_FormatStates(t *testing.T) {
	states := []*model.VehicleState{makeTestState()}
	formatter := &YAMLFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatStates(&buf, states)
	if err != nil {
		t.Fatalf("FormatStates failed: %v", err)
	}

	// Verify it's valid YAML
	var decoded []*model.VehicleState
	if err := yaml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Invalid YAML: %v", err)
	}

	if len(decoded) != 1 {
		t.Errorf("Expected 1 state, got %d", len(decoded))
	}
}

func TestCSVFormatter_FormatState(t *testing.T) {
	state := makeTestState()
	formatter := &CSVFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatState(&buf, state)
	if err != nil {
		t.Fatalf("FormatState failed: %v", err)
	}

	output := buf.String()

	// Verify header
	if !strings.Contains(output, "Timestamp") {
		t.Error("CSV missing Timestamp header")
	}
	if !strings.Contains(output, "BatteryLevel") {
		t.Error("CSV missing BatteryLevel header")
	}

	// Verify data row
	if !strings.Contains(output, "vehicle-123") {
		t.Error("CSV missing vehicle ID")
	}
	if !strings.Contains(output, "85.5") {
		t.Error("CSV missing battery level")
	}
}

func TestCSVFormatter_FormatStates(t *testing.T) {
	states := []*model.VehicleState{makeTestState(), makeTestState()}
	formatter := &CSVFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatStates(&buf, states)
	if err != nil {
		t.Fatalf("FormatStates failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + 2 data rows
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
}

func TestTextFormatter_FormatState(t *testing.T) {
	state := makeTestState()
	formatter := &TextFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatState(&buf, state)
	if err != nil {
		t.Fatalf("FormatState failed: %v", err)
	}

	output := buf.String()

	// Verify key content
	if !strings.Contains(output, "My R1T") {
		t.Error("Text output missing vehicle name")
	}
	if !strings.Contains(output, "85.5%") {
		t.Error("Text output missing battery level")
	}
	if !strings.Contains(output, "250 miles") {
		t.Error("Text output missing range")
	}
	if !strings.Contains(output, "Locked") {
		t.Error("Text output missing lock status")
	}
	if !strings.Contains(output, "Ready Score") {
		t.Error("Text output missing ready score")
	}
}

func TestTextFormatter_FormatStates(t *testing.T) {
	states := []*model.VehicleState{makeTestState(), makeTestState()}
	formatter := &TextFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatStates(&buf, states)
	if err != nil {
		t.Fatalf("FormatStates failed: %v", err)
	}

	output := buf.String()

	// Should have separator between states
	if !strings.Contains(output, "---") {
		t.Error("Text output missing separator between states")
	}
}

func TestTableFormatter_FormatState(t *testing.T) {
	state := makeTestState()
	formatter := &TableFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatState(&buf, state)
	if err != nil {
		t.Fatalf("FormatState failed: %v", err)
	}

	output := buf.String()

	// Verify header
	if !strings.Contains(output, "TIMESTAMP") {
		t.Error("Table missing TIMESTAMP header")
	}
	if !strings.Contains(output, "BATTERY") {
		t.Error("Table missing BATTERY header")
	}

	// Verify data
	if !strings.Contains(output, "85.5%") {
		t.Error("Table missing battery level")
	}
	if !strings.Contains(output, "250mi") {
		t.Error("Table missing range")
	}
}

func TestTableFormatter_FormatStates(t *testing.T) {
	states := []*model.VehicleState{makeTestState(), makeTestState()}
	formatter := &TableFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatStates(&buf, states)
	if err != nil {
		t.Fatalf("FormatStates failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + separator + 2 data rows
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}
}

func TestTableFormatter_EmptyStates(t *testing.T) {
	formatter := &TableFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatStates(&buf, []*model.VehicleState{})
	if err != nil {
		t.Fatalf("FormatStates failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No states") {
		t.Error("Empty table should show 'No states' message")
	}
}

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		format  OutputFormat
		wantErr bool
	}{
		{FormatJSON, false},
		{FormatYAML, false},
		{FormatCSV, false},
		{FormatText, false},
		{FormatTable, false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			formatter, err := NewFormatter(tt.format, true)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if formatter == nil {
				t.Error("Formatter is nil")
			}
		})
	}
}

func TestFormatHelpers(t *testing.T) {
	t.Run("formatFloat", func(t *testing.T) {
		result := formatFloat(123.456, 2)
		if result != "123.46" {
			t.Errorf("Expected 123.46, got %s", result)
		}
	})

	t.Run("formatFloatPtr", func(t *testing.T) {
		f := 123.456
		result := formatFloatPtr(&f, 1)
		if result != "123.5" {
			t.Errorf("Expected 123.5, got %s", result)
		}

		result = formatFloatPtr(nil, 1)
		if result != "" {
			t.Errorf("Expected empty string for nil, got %s", result)
		}
	})

	t.Run("formatBool", func(t *testing.T) {
		if formatBool(true) != "true" {
			t.Error("formatBool(true) should return 'true'")
		}
		if formatBool(false) != "false" {
			t.Error("formatBool(false) should return 'false'")
		}
	})

	t.Run("formatDuration", func(t *testing.T) {
		tests := []struct {
			duration time.Duration
			want     string
		}{
			{30 * time.Minute, "30m"},
			{90 * time.Minute, "1h 30m"},
			{2*time.Hour + 15*time.Minute, "2h 15m"},
			{-1 * time.Minute, "0m"},
		}

		for _, tt := range tests {
			result := formatDuration(tt.duration)
			if result != tt.want {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.want)
			}
		}
	})

	t.Run("formatClosures", func(t *testing.T) {
		allClosed := model.Closures{
			FrontLeft:  model.ClosureStatusClosed,
			FrontRight: model.ClosureStatusClosed,
			RearLeft:   model.ClosureStatusClosed,
			RearRight:  model.ClosureStatusClosed,
		}
		result := formatClosures(allClosed)
		if result != "All closed" {
			t.Errorf("Expected 'All closed', got %s", result)
		}

		oneOpen := model.Closures{
			FrontLeft:  model.ClosureStatusOpen,
			FrontRight: model.ClosureStatusClosed,
			RearLeft:   model.ClosureStatusClosed,
			RearRight:  model.ClosureStatusClosed,
		}
		result = formatClosures(oneOpen)
		if result != "1 open" {
			t.Errorf("Expected '1 open', got %s", result)
		}
	})
}
