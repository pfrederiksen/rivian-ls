package tui

import (
	"strings"
	"testing"

	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/store"
)

func TestNewHealthView(t *testing.T) {
	// Create a temporary store
	tmpStore, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = tmpStore.Close() }()

	view := NewHealthView(tmpStore, "test-vehicle-id")
	if view == nil {
		t.Fatal("NewHealthView returned nil")
	}
	if view.store == nil {
		t.Error("HealthView store is nil")
	}
	if view.vehicleID != "test-vehicle-id" {
		t.Errorf("Expected vehicleID 'test-vehicle-id', got %q", view.vehicleID)
	}
}

func TestHealthViewRender(t *testing.T) {
	tmpStore, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = tmpStore.Close() }()

	view := NewHealthView(tmpStore, "test-vehicle-id")
	state := createTestState()

	output := view.Render(state, 120, 40)

	// Check for key sections
	expectedSections := []string{
		"Vehicle Health",
		"Current Status",
		"Trends",
		"Diagnostics",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Health view missing section: %s", section)
		}
	}
}

func TestHealthViewOnlineStatus(t *testing.T) {
	tmpStore, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = tmpStore.Close() }()

	view := NewHealthView(tmpStore, "test-vehicle-id")

	tests := []struct {
		name         string
		isOnline     bool
		expectedText string
	}{
		{
			name:         "online vehicle",
			isOnline:     true,
			expectedText: "Online",
		},
		{
			name:         "offline vehicle",
			isOnline:     false,
			expectedText: "Offline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			state.IsOnline = tt.isOnline

			output := view.Render(state, 120, 40)

			if !strings.Contains(output, tt.expectedText) {
				t.Errorf("Expected %q in output, got: %s", tt.expectedText, output)
			}
		})
	}
}

func TestHealthViewWithIssues(t *testing.T) {
	tmpStore, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = tmpStore.Close() }()

	view := NewHealthView(tmpStore, "test-vehicle-id")

	tests := []struct {
		name         string
		setupState   func(*model.VehicleState)
		expectedText string
	}{
		{
			name: "critical range",
			setupState: func(s *model.VehicleState) {
				s.RangeStatus = model.RangeStatusCritical
				s.RangeEstimate = 15.0
			},
			expectedText: "Needs Attention",
		},
		{
			name: "healthy vehicle",
			setupState: func(s *model.VehicleState) {
				s.RangeStatus = model.RangeStatusNormal
				s.BatteryLevel = 80.0
				s.ChargeLimit = 80
				s.IsLocked = true // Locked vehicle has no issues
			},
			expectedText: "Healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState()
			tt.setupState(state)

			output := view.Render(state, 120, 40)

			if !strings.Contains(output, tt.expectedText) {
				t.Errorf("Expected %q in output, got: %s", tt.expectedText, output)
			}
		})
	}
}
