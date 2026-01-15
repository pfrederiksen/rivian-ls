package model

import (
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

// Event represents an event that can update the vehicle state.
type Event interface {
	// ApplyTo applies this event to the given state, returning updated state.
	ApplyTo(current *VehicleState) *VehicleState
}

// VehicleListReceived is emitted when we receive the vehicle list from the API.
type VehicleListReceived struct {
	Vehicles []rivian.Vehicle
	VehicleID string // The vehicle we're tracking
}

// ApplyTo updates the state with vehicle identity information.
func (e VehicleListReceived) ApplyTo(current *VehicleState) *VehicleState {
	// Find our vehicle in the list
	for _, v := range e.Vehicles {
		if v.ID == e.VehicleID {
			if current == nil {
				current = &VehicleState{}
			}

			// Update identity fields
			current.VehicleID = v.ID
			current.VIN = v.VIN
			current.Name = v.Name
			current.Model = v.Model
			current.UpdatedAt = time.Now()

			return current
		}
	}

	// Vehicle not found - return current state unchanged
	return current
}

// VehicleStateReceived is emitted when we receive vehicle state from the API.
type VehicleStateReceived struct {
	State *rivian.VehicleState
}

// ApplyTo merges the new vehicle state into the current state.
func (e VehicleStateReceived) ApplyTo(current *VehicleState) *VehicleState {
	if e.State == nil {
		return current
	}

	// Convert Rivian API state to our domain model
	newState := FromRivianVehicleState(e.State)

	// If we have existing state, preserve identity fields that might not be in the state query
	if current != nil {
		if newState.VIN == "" {
			newState.VIN = current.VIN
		}
		if newState.Name == "" {
			newState.Name = current.Name
		}
		if newState.Model == "" {
			newState.Model = current.Model
		}
	}

	return newState
}

// PartialStateUpdate represents a partial update (e.g., from WebSocket).
type PartialStateUpdate struct {
	VehicleID string
	Updates   map[string]interface{} // Field name -> new value
}

// ApplyTo applies partial updates to the current state.
func (e PartialStateUpdate) ApplyTo(current *VehicleState) *VehicleState {
	if current == nil {
		current = &VehicleState{
			VehicleID: e.VehicleID,
		}
	}

	// Make a copy to avoid mutation
	updated := *current
	updated.UpdatedAt = time.Now()

	// Apply updates
	for field, value := range e.Updates {
		switch field {
		case "batteryLevel":
			if v, ok := value.(float64); ok {
				updated.BatteryLevel = v
			}
		case "rangeEstimate":
			if v, ok := value.(float64); ok {
				updated.RangeEstimate = v
				updated.RangeStatus = DetermineRangeStatus(v)
			}
		case "chargeState":
			if v, ok := value.(string); ok {
				updated.ChargeState = ChargeState(v)
			}
		case "isLocked":
			if v, ok := value.(bool); ok {
				updated.IsLocked = v
			}
		case "cabinTemp":
			if v, ok := value.(float64); ok {
				updated.CabinTemp = &v
			}
		// Add more fields as needed for WebSocket updates
		}
	}

	return &updated
}

// Reducer processes events and produces new state.
type Reducer struct {
	currentState *VehicleState
}

// NewReducer creates a new state reducer.
func NewReducer() *Reducer {
	return &Reducer{}
}

// Dispatch processes an event and updates the state.
func (r *Reducer) Dispatch(event Event) *VehicleState {
	r.currentState = event.ApplyTo(r.currentState)
	return r.currentState
}

// GetState returns the current state (read-only).
func (r *Reducer) GetState() *VehicleState {
	if r.currentState == nil {
		return nil
	}
	// Return a copy to prevent external mutation
	state := *r.currentState
	return &state
}

// Reset clears the current state.
func (r *Reducer) Reset() {
	r.currentState = nil
}
