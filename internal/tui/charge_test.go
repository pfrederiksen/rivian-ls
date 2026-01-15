package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/pfrederiksen/rivian-ls/internal/model"
)

func TestNewChargeView(t *testing.T) {
	view := NewChargeView()
	if view == nil {
		t.Fatal("NewChargeView returned nil")
	}
}

func TestChargeViewRender(t *testing.T) {
	view := NewChargeView()
	state := createTestState()

	output := view.Render(state, 120, 40)

	// Check for key sections
	expectedSections := []string{
		"Charging",
		"Charge Complete",
		"Battery Details",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Charge view missing section: %s", section)
		}
	}
}

func TestRenderChargingStatus(t *testing.T) {
	view := NewChargeView()

	tests := []struct {
		name         string
		chargeState  model.ChargeState
		expectedText string
	}{
		{
			name:         "charging",
			chargeState:  model.ChargeStateCharging,
			expectedText: "Charging",
		},
		{
			name:         "complete",
			chargeState:  model.ChargeStateComplete,
			expectedText: "Charge Complete",
		},
		{
			name:         "disconnected",
			chargeState:  model.ChargeStateDisconnected,
			expectedText: "Disconnected",
		},
		{
			name:         "not charging",
			chargeState:  model.ChargeStateNotCharging,
			expectedText: "Not Charging",
		},
	}

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			state.ChargeState = tt.chargeState

			output := view.renderChargingStatus(state, sectionStyle, labelStyle, valueStyle)

			if !strings.Contains(output, tt.expectedText) {
				t.Errorf("Expected %q in output, got: %s", tt.expectedText, output)
			}
		})
	}
}

func TestRenderChargingStatusWithRate(t *testing.T) {
	view := NewChargeView()
	state := createTestState()
	state.ChargeState = model.ChargeStateCharging
	chargingRate := 11.5
	state.ChargingRate = &chargingRate

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderChargingStatus(state, sectionStyle, labelStyle, valueStyle)

	if !strings.Contains(output, "11.5 kW") {
		t.Errorf("Expected charging rate in output, got: %s", output)
	}
}

func TestRenderChargingStatusWithTimeToCharge(t *testing.T) {
	view := NewChargeView()
	state := createTestState()
	state.ChargeState = model.ChargeStateCharging
	state.UpdatedAt = time.Now()
	futureTime := time.Now().Add(2*time.Hour + 30*time.Minute)
	state.TimeToCharge = &futureTime

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderChargingStatus(state, sectionStyle, labelStyle, valueStyle)

	// Should show time remaining
	if !strings.Contains(output, "2h 30m") {
		t.Errorf("Expected time remaining in output, got: %s", output)
	}
}

func TestRenderBatteryDetails(t *testing.T) {
	view := NewChargeView()
	state := createTestState()
	state.BatteryLevel = 69.9
	state.ChargeLimit = 70

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderBatteryDetails(state, sectionStyle, labelStyle, valueStyle)

	expectedContent := []string{
		"Battery Details",
		"Current Level:",
		"69.9%",
		"Charge Limit:",
		"70%",
		"Range:",
		"196 mi",
	}

	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Battery details missing %q, got: %s", content, output)
		}
	}
}

func TestRenderBatteryDetailsAtLimit(t *testing.T) {
	view := NewChargeView()
	state := createTestState()
	state.BatteryLevel = 80.0
	state.ChargeLimit = 80

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderBatteryDetails(state, sectionStyle, labelStyle, valueStyle)

	if !strings.Contains(output, "At limit") {
		t.Errorf("Expected 'At limit' message, got: %s", output)
	}
}

func TestRenderBatteryDetailsBelowLimit(t *testing.T) {
	view := NewChargeView()
	state := createTestState()
	state.BatteryLevel = 60.0
	state.ChargeLimit = 80

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderBatteryDetails(state, sectionStyle, labelStyle, valueStyle)

	if !strings.Contains(output, "To Limit:") {
		t.Errorf("Expected 'To Limit' label, got: %s", output)
	}
	if !strings.Contains(output, "+20.0%") {
		t.Errorf("Expected +20.0%% difference, got: %s", output)
	}
}

func TestGetChargingRecommendations(t *testing.T) {
	view := NewChargeView()

	tests := []struct {
		name             string
		setupState       func(*model.VehicleState)
		expectedMinRecs  int
		expectedContains string
	}{
		{
			name: "critical battery",
			setupState: func(s *model.VehicleState) {
				s.RangeStatus = model.RangeStatusCritical
				s.RangeEstimate = 20
			},
			expectedMinRecs:  1,
			expectedContains: "Critical battery",
		},
		{
			name: "low battery not charging",
			setupState: func(s *model.VehicleState) {
				s.RangeStatus = model.RangeStatusLow
				s.RangeEstimate = 40
				s.ChargeState = model.ChargeStateDisconnected
			},
			expectedMinRecs:  1,
			expectedContains: "Low battery",
		},
		{
			name: "below charge limit",
			setupState: func(s *model.VehicleState) {
				s.BatteryLevel = 60
				s.ChargeLimit = 80
				s.ChargeState = model.ChargeStateDisconnected
			},
			expectedMinRecs:  1,
			expectedContains: "below limit",
		},
		{
			name: "battery above 85%",
			setupState: func(s *model.VehicleState) {
				s.BatteryLevel = 90
			},
			expectedMinRecs:  1,
			expectedContains: "above 85%",
		},
		{
			name: "charge complete",
			setupState: func(s *model.VehicleState) {
				s.ChargeState = model.ChargeStateComplete
				s.BatteryLevel = 80
				s.ChargeLimit = 80
			},
			expectedMinRecs:  1,
			expectedContains: "Charge complete",
		},
		{
			name: "no issues",
			setupState: func(s *model.VehicleState) {
				s.RangeStatus = model.RangeStatusNormal
				s.BatteryLevel = 75
				s.ChargeLimit = 80
				s.ChargeState = model.ChargeStateCharging
			},
			expectedMinRecs:  0,
			expectedContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			tt.setupState(state)

			recs := view.getChargingRecommendations(state)

			if len(recs) < tt.expectedMinRecs {
				t.Errorf("Expected at least %d recommendations, got %d", tt.expectedMinRecs, len(recs))
			}

			if tt.expectedContains != "" {
				found := false
				for _, rec := range recs {
					if strings.Contains(rec.message, tt.expectedContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected recommendation containing %q, got: %+v", tt.expectedContains, recs)
				}
			}
		})
	}
}

func TestRenderRecommendations(t *testing.T) {
	view := NewChargeView()
	state := createTestState()
	state.RangeStatus = model.RangeStatusCritical
	state.RangeEstimate = 15

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderRecommendations(state, sectionStyle, labelStyle, valueStyle)

	if !strings.Contains(output, "Recommendations") {
		t.Errorf("Expected recommendations header, got: %s", output)
	}
	if !strings.Contains(output, "Critical battery") {
		t.Errorf("Expected critical battery warning, got: %s", output)
	}
}

func TestRenderRecommendationsEmpty(t *testing.T) {
	view := NewChargeView()
	state := createTestState()
	state.RangeStatus = model.RangeStatusNormal
	state.BatteryLevel = 75
	state.ChargeLimit = 80
	state.ChargeState = model.ChargeStateCharging

	sectionStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()

	output := view.renderRecommendations(state, sectionStyle, labelStyle, valueStyle)

	// Should return empty string when no recommendations
	if output != "" {
		t.Errorf("Expected empty output for no recommendations, got: %s", output)
	}
}
