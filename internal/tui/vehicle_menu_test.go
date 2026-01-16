package tui

import (
	"strings"
	"testing"

	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

func TestVehicleMenu_HandleKey(t *testing.T) {
	vehicles := []rivian.Vehicle{
		{ID: "1", Name: "Test 1", Model: "R1T", VIN: "1234567890"},
		{ID: "2", Name: "Test 2", Model: "R1S", VIN: "0987654321"},
		{ID: "3", Name: "Test 3", Model: "R1T", VIN: "1111111111"},
	}

	tests := []struct {
		name          string
		initialIndex  int
		key           string
		expectedIndex int
		expectedDone  bool
	}{
		{
			name:          "down arrow from first",
			initialIndex:  0,
			key:           "down",
			expectedIndex: 1,
			expectedDone:  false,
		},
		{
			name:          "up arrow from second",
			initialIndex:  1,
			key:           "up",
			expectedIndex: 0,
			expectedDone:  false,
		},
		{
			name:          "up arrow at first (no change)",
			initialIndex:  0,
			key:           "up",
			expectedIndex: 0,
			expectedDone:  false,
		},
		{
			name:          "down arrow at last (no change)",
			initialIndex:  2,
			key:           "down",
			expectedIndex: 2,
			expectedDone:  false,
		},
		{
			name:          "j key (vim down)",
			initialIndex:  0,
			key:           "j",
			expectedIndex: 1,
			expectedDone:  false,
		},
		{
			name:          "k key (vim up)",
			initialIndex:  1,
			key:           "k",
			expectedIndex: 0,
			expectedDone:  false,
		},
		{
			name:          "number 1 selects first",
			initialIndex:  2,
			key:           "1",
			expectedIndex: 0,
			expectedDone:  false,
		},
		{
			name:          "number 2 selects second",
			initialIndex:  0,
			key:           "2",
			expectedIndex: 1,
			expectedDone:  false,
		},
		{
			name:          "number 3 selects third",
			initialIndex:  0,
			key:           "3",
			expectedIndex: 2,
			expectedDone:  false,
		},
		{
			name:          "enter confirms selection",
			initialIndex:  1,
			key:           "enter",
			expectedIndex: 1,
			expectedDone:  true,
		},
		{
			name:          "esc cancels",
			initialIndex:  1,
			key:           "esc",
			expectedIndex: -1,
			expectedDone:  true,
		},
		{
			name:          "q cancels",
			initialIndex:  1,
			key:           "q",
			expectedIndex: -1,
			expectedDone:  true,
		},
		{
			name:          "invalid number key (out of range)",
			initialIndex:  0,
			key:           "9",
			expectedIndex: 0,
			expectedDone:  false,
		},
		{
			name:          "unknown key (no change)",
			initialIndex:  1,
			key:           "x",
			expectedIndex: 1,
			expectedDone:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			menu := NewVehicleMenu(vehicles, tt.initialIndex, nil)
			gotIndex, gotDone := menu.HandleKey(tt.key)

			if gotIndex != tt.expectedIndex {
				t.Errorf("HandleKey() index = %v, want %v", gotIndex, tt.expectedIndex)
			}
			if gotDone != tt.expectedDone {
				t.Errorf("HandleKey() done = %v, want %v", gotDone, tt.expectedDone)
			}
		})
	}
}

func TestVehicleMenu_Render(t *testing.T) {
	vehicles := []rivian.Vehicle{
		{ID: "1", Name: "Road Trip", Model: "R1T", VIN: "1234567890ABCDEF"},
		{ID: "2", Name: "Family Hauler", Model: "R1S", VIN: "FEDCBA0987654321"},
	}

	// Create some test states
	batteryLevel85 := 85.0
	batteryLevel62 := 62.0
	states := map[string]*model.VehicleState{
		"1": {
			VehicleID:    "1",
			BatteryLevel: batteryLevel85,
			IsOnline:     true,
		},
		"2": {
			VehicleID:    "2",
			BatteryLevel: batteryLevel62,
			IsOnline:     false,
		},
	}

	tests := []struct {
		name          string
		selectedIndex int
		width         int
		height        int
		checkContent  []string // Strings that should appear in output
	}{
		{
			name:          "first vehicle selected",
			selectedIndex: 0,
			width:         80,
			height:        24,
			checkContent: []string{
				"Select Vehicle",
				"Road Trip",
				"R1T",
				"[85%]",
				"🟢",
				"Family Hauler",
				"R1S",
				"[62%]",
				"🔴",
				"[↑/↓] Navigate",
				"[Enter] Confirm",
			},
		},
		{
			name:          "second vehicle selected",
			selectedIndex: 1,
			width:         80,
			height:        24,
			checkContent: []string{
				"Select Vehicle",
				"Road Trip",
				"Family Hauler",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			menu := NewVehicleMenu(vehicles, tt.selectedIndex, states)
			output := menu.Render(tt.width, tt.height)

			// Check that all expected content appears
			for _, content := range tt.checkContent {
				if !strings.Contains(output, content) {
					t.Errorf("Render() missing expected content %q\nGot:\n%s", content, output)
				}
			}
		})
	}
}

func TestVehicleMenu_EmptyVehicles(t *testing.T) {
	// Test with empty vehicles list (edge case)
	menu := NewVehicleMenu([]rivian.Vehicle{}, 0, nil)

	// Try to navigate down (should not crash, stays at 0 due to bounds check)
	index, done := menu.HandleKey("down")
	if index != 0 || done {
		t.Errorf("HandleKey on empty menu = (%v, %v), want (0, false)", index, done)
	}

	// Try to render (should not crash)
	output := menu.Render(80, 24)
	if !strings.Contains(output, "Select Vehicle") {
		t.Error("Render should still show title even with empty vehicles")
	}
}

func TestVehicleMenu_SingleVehicle(t *testing.T) {
	// Test with single vehicle
	vehicles := []rivian.Vehicle{
		{ID: "1", Name: "Only One", Model: "R1T", VIN: "1234567890"},
	}

	menu := NewVehicleMenu(vehicles, 0, nil)

	// Try to navigate down (should not change)
	index, done := menu.HandleKey("down")
	if index != 0 || done {
		t.Errorf("HandleKey down on single vehicle = (%v, %v), want (0, false)", index, done)
	}

	// Try to navigate up (should not change)
	index, done = menu.HandleKey("up")
	if index != 0 || done {
		t.Errorf("HandleKey up on single vehicle = (%v, %v), want (0, false)", index, done)
	}

	// Confirm should work
	index, done = menu.HandleKey("enter")
	if index != 0 || !done {
		t.Errorf("HandleKey enter = (%v, %v), want (0, true)", index, done)
	}
}

func TestVehicleMenu_MissingState(t *testing.T) {
	// Test rendering when state is missing for a vehicle
	vehicles := []rivian.Vehicle{
		{ID: "1", Name: "Has State", Model: "R1T", VIN: "1111111111"},
		{ID: "2", Name: "No State", Model: "R1S", VIN: "2222222222"},
	}

	batteryLevel := 50.0
	states := map[string]*model.VehicleState{
		"1": {
			VehicleID:    "1",
			BatteryLevel: batteryLevel,
			IsOnline:     true,
		},
		// "2" is missing
	}

	menu := NewVehicleMenu(vehicles, 0, states)
	output := menu.Render(80, 24)

	// Should show both vehicles
	if !strings.Contains(output, "Has State") {
		t.Error("Missing vehicle with state")
	}
	if !strings.Contains(output, "No State") {
		t.Error("Missing vehicle without state")
	}

	// Should show [--] for missing state
	if !strings.Contains(output, "[--]") {
		t.Error("Should show [--] for vehicle without state")
	}
}
