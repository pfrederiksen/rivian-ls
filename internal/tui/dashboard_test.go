package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/pfrederiksen/rivian-ls/internal/model"
)

func TestNewDashboardView(t *testing.T) {
	view := NewDashboardView()
	if view == nil {
		t.Fatal("NewDashboardView returned nil")
	}
}

func TestDashboardRender(t *testing.T) {
	view := NewDashboardView()
	state := createTestState()

	output := view.Render(state, 120, 40)

	// Check for key sections
	expectedSections := []string{
		"Dashboard",
		"Battery & Range",
		"Charging",
		"Security",
		"Tire Status",
		"Climate & Travel",
		"Battery Stats",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Dashboard output missing section: %s", section)
		}
	}
}

func TestRenderBatterySection(t *testing.T) {
	view := NewDashboardView()
	state := createTestState()

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderBatterySection(state, sectionStyle, labelStyle, valueStyle)

	// Check for battery information
	expectedContent := []string{
		"Battery:",
		"70.0%",
		"Range:",
		"196 mi",
		"Charge Limit:",
		"70%",
	}

	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Battery section missing content: %s\nGot: %s", content, output)
		}
	}
}

func TestRenderChargingSection(t *testing.T) {
	view := NewDashboardView()

	tests := []struct {
		name          string
		chargeState   model.ChargeState
		expectedText  string
		expectedEmoji string
	}{
		{
			name:          "charging",
			chargeState:   model.ChargeStateCharging,
			expectedText:  "Charging",
			expectedEmoji: "⚡",
		},
		{
			name:          "complete",
			chargeState:   model.ChargeStateComplete,
			expectedText:  "Complete",
			expectedEmoji: "✓",
		},
		{
			name:          "disconnected",
			chargeState:   model.ChargeStateDisconnected,
			expectedText:  "Disconnected",
			expectedEmoji: "🔌",
		},
		{
			name:          "scheduled",
			chargeState:   model.ChargeStateScheduled,
			expectedText:  "Scheduled",
			expectedEmoji: "⏱",
		},
	}

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			state.ChargeState = tt.chargeState

			output := view.renderChargingSection(state, sectionStyle, labelStyle, valueStyle)

			if !strings.Contains(output, tt.expectedText) {
				t.Errorf("Expected charging text %q, got: %s", tt.expectedText, output)
			}
			if !strings.Contains(output, tt.expectedEmoji) {
				t.Errorf("Expected emoji %q, got: %s", tt.expectedEmoji, output)
			}
		})
	}
}

func TestRenderSecuritySection(t *testing.T) {
	view := NewDashboardView()

	tests := []struct {
		name         string
		isLocked     bool
		expectedText string
	}{
		{
			name:         "locked",
			isLocked:     true,
			expectedText: "Locked",
		},
		{
			name:         "unlocked",
			isLocked:     false,
			expectedText: "Unlocked",
		},
	}

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			state.IsLocked = tt.isLocked

			output := view.renderSecuritySection(state, sectionStyle, labelStyle, valueStyle)

			if !strings.Contains(output, tt.expectedText) {
				t.Errorf("Expected security text %q, got: %s", tt.expectedText, output)
			}
		})
	}
}

func TestRenderTirePressures(t *testing.T) {
	view := NewDashboardView()

	tests := []struct {
		name   string
		status model.TirePressureStatus
	}{
		{name: "OK status", status: model.TirePressureStatusOK},
		{name: "Low status", status: model.TirePressureStatusLow},
		{name: "High status", status: model.TirePressureStatusHigh},
		{name: "Unknown status", status: model.TirePressureStatusUnknown},
	}

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			state.TirePressures.FrontLeftStatus = tt.status
			state.TirePressures.FrontRightStatus = tt.status
			state.TirePressures.RearLeftStatus = tt.status
			state.TirePressures.RearRightStatus = tt.status

			output := view.renderTirePressures(state, sectionStyle, labelStyle, valueStyle)

			if !strings.Contains(output, "Tire Status") {
				t.Errorf("Missing tire status header, got: %s", output)
			}

			// Check for tire positions
			positions := []string{"Front Left", "Front Right", "Rear Left", "Rear Right"}
			for _, pos := range positions {
				if !strings.Contains(output, pos) {
					t.Errorf("Missing tire position %q, got: %s", pos, output)
				}
			}
		})
	}
}

func TestRenderVehicleInfo(t *testing.T) {
	view := NewDashboardView()
	state := createTestState()
	state.BatteryCapacity = 140.5 // Set calculated capacity

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderVehicleInfo(state, sectionStyle, labelStyle, valueStyle)

	// Check for battery stats
	expectedContent := []string{
		"Battery Stats",
		"Capacity:",
		"140.5 kWh",
		"Current Energy:",
		"Efficiency:",
		"mi/kWh",
		"mi/%:",
	}

	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Vehicle info missing content: %s\nGot: %s", content, output)
		}
	}
}

func TestRenderBatteryBar(t *testing.T) {
	view := NewDashboardView()

	tests := []struct {
		name  string
		level float64
		width int
	}{
		{name: "full battery", level: 100, width: 20},
		{name: "half battery", level: 50, width: 20},
		{name: "low battery", level: 15, width: 20},
		{name: "critical battery", level: 5, width: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := view.renderBatteryBar(tt.level, tt.width)

			// Bar should contain brackets and fill characters
			if !strings.Contains(bar, "[") || !strings.Contains(bar, "]") {
				t.Errorf("Battery bar missing brackets, got: %s", bar)
			}

			// Check length is roughly correct (accounting for ANSI codes)
			if len(bar) < tt.width {
				t.Errorf("Battery bar too short for width %d, got: %s", tt.width, bar)
			}
		})
	}
}

func TestRenderStatsSection(t *testing.T) {
	view := NewDashboardView()

	tests := []struct {
		name       string
		cabinTemp  *float64
		hasContent string
	}{
		{
			name:       "with temperature",
			cabinTemp:  ptr(72.5),
			hasContent: "72.5°F",
		},
		{
			name:       "no temperature",
			cabinTemp:  nil,
			hasContent: "Odometer",
		},
	}

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			state.CabinTemp = tt.cabinTemp

			output := view.renderStatsSection(state, sectionStyle, labelStyle, valueStyle)

			if !strings.Contains(output, tt.hasContent) {
				t.Errorf("Stats section missing %q, got: %s", tt.hasContent, output)
			}
		})
	}
}

func TestRenderIssues(t *testing.T) {
	view := NewDashboardView()

	tests := []struct {
		name         string
		rangeStatus  model.RangeStatus
		batteryLevel float64
		chargeLimit  int
		hasIssues    bool
	}{
		{
			name:         "no issues",
			rangeStatus:  model.RangeStatusNormal,
			batteryLevel: 80,
			chargeLimit:  80,
			hasIssues:    false,
		},
		{
			name:         "critical battery",
			rangeStatus:  model.RangeStatusCritical,
			batteryLevel: 10,
			chargeLimit:  80,
			hasIssues:    true,
		},
		{
			name:         "below charge limit",
			rangeStatus:  model.RangeStatusNormal,
			batteryLevel: 60,
			chargeLimit:  80,
			hasIssues:    true,
		},
	}

	sectionStyle := lipgloss.NewStyle()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			state.RangeStatus = tt.rangeStatus
			state.BatteryLevel = tt.batteryLevel
			state.ChargeLimit = tt.chargeLimit

			// For "no issues" case, lock the vehicle to truly have no issues
			if !tt.hasIssues {
				state.IsLocked = true
			}

			output := view.renderIssues(state, sectionStyle)

			if tt.hasIssues && output == "" {
				t.Error("Expected issues section but got empty string")
			}
			if !tt.hasIssues && output != "" {
				t.Errorf("Expected no issues but got: %s", output)
			}
		})
	}
}

// Helper function to create a test state
func createTestState() *model.VehicleState {
	cabinTemp := 72.0
	return &model.VehicleState{
		VehicleID:       "test-vehicle-id",
		VIN:             "TEST123456",
		Name:            "Test Vehicle",
		Model:           "R1T",
		UpdatedAt:       time.Now(),
		BatteryLevel:    70.0,
		BatteryCapacity: 140.5,
		RangeEstimate:   196.0,
		ChargeState:     model.ChargeStateComplete,
		ChargeLimit:     70,
		IsLocked:        false,
		IsOnline:        true,
		Odometer:        33557.9,
		CabinTemp:       &cabinTemp,
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
		TirePressures: model.TirePressures{
			FrontLeftStatus:  model.TirePressureStatusOK,
			FrontRightStatus: model.TirePressureStatusOK,
			RearLeftStatus:   model.TirePressureStatusOK,
			RearRightStatus:  model.TirePressureStatusOK,
		},
		RangeStatus: model.RangeStatusNormal,
		Location: &model.Location{
			Latitude:  36.1743,
			Longitude: -115.3663,
			UpdatedAt: time.Now(),
		},
	}
}

// Helper to create pointer to float64
func ptr(f float64) *float64 {
	return &f
}
