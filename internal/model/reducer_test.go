package model

import (
	"testing"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

func TestReducer_VehicleListReceived(t *testing.T) {
	reducer := NewReducer()

	vehicles := []rivian.Vehicle{
		{
			ID:    "vehicle-1",
			VIN:   "VIN123",
			Name:  "My R1T",
			Model: "R1T",
			Year:  2023,
		},
		{
			ID:    "vehicle-2",
			VIN:   "VIN456",
			Name:  "My R1S",
			Model: "R1S",
			Year:  2024,
		},
	}

	event := VehicleListReceived{
		Vehicles:  vehicles,
		VehicleID: "vehicle-1",
	}

	state := reducer.Dispatch(event)

	if state.VehicleID != "vehicle-1" {
		t.Errorf("VehicleID = %v, want %v", state.VehicleID, "vehicle-1")
	}
	if state.VIN != "VIN123" {
		t.Errorf("VIN = %v, want %v", state.VIN, "VIN123")
	}
	if state.Name != "My R1T" {
		t.Errorf("Name = %v, want %v", state.Name, "My R1T")
	}
	if state.Model != "R1T" {
		t.Errorf("Model = %v, want %v", state.Model, "R1T")
	}
}

func TestReducer_VehicleListReceived_VehicleNotFound(t *testing.T) {
	reducer := NewReducer()

	// Set up initial state
	initial := &VehicleState{
		VehicleID: "vehicle-1",
		Name:      "Initial",
	}
	reducer.currentState = initial

	vehicles := []rivian.Vehicle{
		{ID: "vehicle-2", Name: "Other Vehicle"},
	}

	event := VehicleListReceived{
		Vehicles:  vehicles,
		VehicleID: "vehicle-1", // Not in list
	}

	state := reducer.Dispatch(event)

	// Should return current state unchanged
	if state.Name != "Initial" {
		t.Errorf("State should be unchanged, got Name = %v", state.Name)
	}
}

func TestReducer_VehicleStateReceived(t *testing.T) {
	reducer := NewReducer()

	// Set up initial state with identity
	reducer.currentState = &VehicleState{
		VehicleID: "vehicle-1",
		VIN:       "VIN123",
		Name:      "My R1T",
		Model:     "R1T",
	}

	cabinTemp := 72.0
	rivState := &rivian.VehicleState{
		VehicleID:     "vehicle-1",
		UpdatedAt:     time.Now(),
		BatteryLevel:  85.5,
		RangeEstimate: 250.0,
		ChargeState:   rivian.ChargeStateCharging,
		IsLocked:      true,
		IsOnline:      true,
		CabinTemp:     &cabinTemp,
		Doors: rivian.ClosureState{
			FrontLeft:  rivian.ClosureStatusClosed,
			FrontRight: rivian.ClosureStatusClosed,
			RearLeft:   rivian.ClosureStatusClosed,
			RearRight:  rivian.ClosureStatusClosed,
		},
	}

	event := VehicleStateReceived{State: rivState}
	state := reducer.Dispatch(event)

	if state.BatteryLevel != 85.5 {
		t.Errorf("BatteryLevel = %v, want %v", state.BatteryLevel, 85.5)
	}
	if state.RangeEstimate != 250.0 {
		t.Errorf("RangeEstimate = %v, want %v", state.RangeEstimate, 250.0)
	}
	if state.ChargeState != ChargeStateCharging {
		t.Errorf("ChargeState = %v, want %v", state.ChargeState, ChargeStateCharging)
	}

	// Verify identity fields are preserved
	if state.VIN != "VIN123" {
		t.Errorf("VIN should be preserved, got %v", state.VIN)
	}
	if state.Name != "My R1T" {
		t.Errorf("Name should be preserved, got %v", state.Name)
	}
	if state.Model != "R1T" {
		t.Errorf("Model should be preserved, got %v", state.Model)
	}
}

func TestReducer_PartialStateUpdate(t *testing.T) {
	reducer := NewReducer()

	// Set up initial state
	reducer.currentState = &VehicleState{
		VehicleID:     "vehicle-1",
		BatteryLevel:  80.0,
		RangeEstimate: 200.0,
		ChargeState:   ChargeStateNotCharging,
	}

	event := PartialStateUpdate{
		VehicleID: "vehicle-1",
		Updates: map[string]interface{}{
			"batteryLevel":  85.5,
			"rangeEstimate": 250.0,
			"chargeState":   "charging",
		},
	}

	state := reducer.Dispatch(event)

	if state.BatteryLevel != 85.5 {
		t.Errorf("BatteryLevel = %v, want %v", state.BatteryLevel, 85.5)
	}
	if state.RangeEstimate != 250.0 {
		t.Errorf("RangeEstimate = %v, want %v", state.RangeEstimate, 250.0)
	}
	if state.ChargeState != ChargeStateCharging {
		t.Errorf("ChargeState = %v, want %v", state.ChargeState, ChargeStateCharging)
	}
	if state.RangeStatus != RangeStatusNormal {
		t.Errorf("RangeStatus should be updated to %v, got %v", RangeStatusNormal, state.RangeStatus)
	}
}

func TestReducer_GetState(t *testing.T) {
	reducer := NewReducer()

	// Initially nil
	if state := reducer.GetState(); state != nil {
		t.Error("Initial state should be nil")
	}

	// Set state
	reducer.currentState = &VehicleState{
		VehicleID:    "vehicle-1",
		BatteryLevel: 85.5,
	}

	// GetState should return a copy
	state := reducer.GetState()
	if state == nil {
		t.Fatal("GetState returned nil")
	}
	if state.VehicleID != "vehicle-1" {
		t.Errorf("VehicleID = %v, want %v", state.VehicleID, "vehicle-1")
	}

	// Mutating the returned state should not affect internal state
	state.BatteryLevel = 50.0
	if reducer.currentState.BatteryLevel != 85.5 {
		t.Error("GetState should return a copy, not the original")
	}
}

func TestReducer_Reset(t *testing.T) {
	reducer := NewReducer()

	reducer.currentState = &VehicleState{VehicleID: "vehicle-1"}
	reducer.Reset()

	if reducer.currentState != nil {
		t.Error("Reset should clear state")
	}
	if state := reducer.GetState(); state != nil {
		t.Error("GetState should return nil after reset")
	}
}

func TestReducer_MultipleEvents(t *testing.T) {
	reducer := NewReducer()

	// 1. Receive vehicle list
	event1 := VehicleListReceived{
		Vehicles: []rivian.Vehicle{
			{
				ID:    "vehicle-1",
				VIN:   "VIN123",
				Name:  "My R1T",
				Model: "R1T",
			},
		},
		VehicleID: "vehicle-1",
	}
	reducer.Dispatch(event1)

	// 2. Receive vehicle state
	cabinTemp := 72.0
	event2 := VehicleStateReceived{
		State: &rivian.VehicleState{
			VehicleID:     "vehicle-1",
			BatteryLevel:  85.5,
			RangeEstimate: 250.0,
			CabinTemp:     &cabinTemp,
		},
	}
	reducer.Dispatch(event2)

	// 3. Partial update
	event3 := PartialStateUpdate{
		VehicleID: "vehicle-1",
		Updates: map[string]interface{}{
			"batteryLevel": 90.0,
		},
	}
	state := reducer.Dispatch(event3)

	// Verify final state has all data
	if state.VehicleID != "vehicle-1" {
		t.Error("VehicleID missing")
	}
	if state.VIN != "VIN123" {
		t.Error("VIN missing")
	}
	if state.Name != "My R1T" {
		t.Error("Name missing")
	}
	if state.BatteryLevel != 90.0 {
		t.Errorf("BatteryLevel = %v, want %v", state.BatteryLevel, 90.0)
	}
	if state.RangeEstimate != 250.0 {
		t.Errorf("RangeEstimate = %v, want %v", state.RangeEstimate, 250.0)
	}
	if state.CabinTemp == nil || *state.CabinTemp != 72.0 {
		t.Error("CabinTemp should be preserved")
	}
}
