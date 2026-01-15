package model

import (
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

// VehicleState represents the complete domain model for a Rivian vehicle.
// This is the "single source of truth" used by both TUI and CLI.
type VehicleState struct {
	// Identity
	VehicleID string
	VIN       string
	Name      string
	Model     string

	// Timestamp of last update
	UpdatedAt time.Time

	// Battery & Charging
	BatteryLevel    float64   // Percentage (0-100)
	BatteryCapacity float64   // kWh (total capacity)
	RangeEstimate   float64   // Miles remaining
	ChargeState     ChargeState
	ChargeLimit     int       // Percentage (0-100)
	ChargingRate    *float64  // kW (nil if not charging)
	TimeToCharge    *time.Time // When charging will complete (nil if not charging)

	// Location
	Location *Location

	// Climate
	CabinTemp    *float64 // Fahrenheit
	ExteriorTemp *float64 // Fahrenheit

	// Security & Status
	IsLocked bool
	IsOnline bool
	Odometer float64 // Miles

	// Closures
	Doors        Closures
	Windows      Closures
	Frunk        ClosureStatus
	Liftgate     ClosureStatus
	TonneauCover *ClosureStatus // R1T only

	// Tires
	TirePressures TirePressures

	// Derived Metrics (calculated by insights.go)
	ReadyScore  *float64 // 0-100 score of "readiness to drive"
	RangeStatus RangeStatus
}

// Location represents GPS coordinates.
type Location struct {
	Latitude  float64
	Longitude float64
	UpdatedAt time.Time
}

// ChargeState represents the charging status.
type ChargeState string

const (
	ChargeStateUnknown      ChargeState = "unknown"
	ChargeStateCharging     ChargeState = "charging"
	ChargeStateComplete     ChargeState = "complete"
	ChargeStateScheduled    ChargeState = "scheduled"
	ChargeStateDisconnected ChargeState = "disconnected"
	ChargeStateNotCharging  ChargeState = "not_charging"
)

// ClosureStatus represents the status of a closure (door, window, etc).
type ClosureStatus string

const (
	ClosureStatusUnknown ClosureStatus = "unknown"
	ClosureStatusOpen    ClosureStatus = "open"
	ClosureStatusClosed  ClosureStatus = "closed"
)

// Closures represents a set of 4 closures (doors or windows).
type Closures struct {
	FrontLeft  ClosureStatus
	FrontRight ClosureStatus
	RearLeft   ClosureStatus
	RearRight  ClosureStatus
}

// AllClosed returns true if all closures are closed.
func (c Closures) AllClosed() bool {
	return c.FrontLeft == ClosureStatusClosed &&
		c.FrontRight == ClosureStatusClosed &&
		c.RearLeft == ClosureStatusClosed &&
		c.RearRight == ClosureStatusClosed
}

// AnyClosed returns true if any closure is closed.
func (c Closures) AnyClosed() bool {
	return c.FrontLeft == ClosureStatusClosed ||
		c.FrontRight == ClosureStatusClosed ||
		c.RearLeft == ClosureStatusClosed ||
		c.RearRight == ClosureStatusClosed
}

// AnyOpen returns true if any closure is open.
func (c Closures) AnyOpen() bool {
	return c.FrontLeft == ClosureStatusOpen ||
		c.FrontRight == ClosureStatusOpen ||
		c.RearLeft == ClosureStatusOpen ||
		c.RearRight == ClosureStatusOpen
}

// TirePressures represents tire pressure readings.
type TirePressures struct {
	FrontLeft  float64 // PSI
	FrontRight float64 // PSI
	RearLeft   float64 // PSI
	RearRight  float64 // PSI
	UpdatedAt  time.Time
}

// AnyLow returns true if any tire is below the threshold (typically 30 PSI).
func (t TirePressures) AnyLow(threshold float64) bool {
	return t.FrontLeft < threshold ||
		t.FrontRight < threshold ||
		t.RearLeft < threshold ||
		t.RearRight < threshold
}

// RangeStatus represents the range warning level.
type RangeStatus string

const (
	RangeStatusUnknown  RangeStatus = "unknown"
	RangeStatusCritical RangeStatus = "critical" // < 25 miles
	RangeStatusLow      RangeStatus = "low"      // < 50 miles
	RangeStatusNormal   RangeStatus = "normal"   // >= 50 miles
)

// DetermineRangeStatus calculates range status based on miles remaining.
func DetermineRangeStatus(miles float64) RangeStatus {
	switch {
	case miles < 25:
		return RangeStatusCritical
	case miles < 50:
		return RangeStatusLow
	default:
		return RangeStatusNormal
	}
}

// FromRivianVehicle converts a Rivian API Vehicle to our domain model (partial).
func FromRivianVehicle(v rivian.Vehicle) *VehicleState {
	return &VehicleState{
		VehicleID: v.ID,
		VIN:       v.VIN,
		Name:      v.Name,
		Model:     v.Model,
		UpdatedAt: time.Now(),
	}
}

// FromRivianVehicleState converts Rivian API VehicleState to our domain model.
func FromRivianVehicleState(v *rivian.VehicleState) *VehicleState {
	state := &VehicleState{
		VehicleID:       v.VehicleID,
		UpdatedAt:       v.UpdatedAt,
		BatteryLevel:    v.BatteryLevel,
		BatteryCapacity: v.BatteryCapacity,
		RangeEstimate:   v.RangeEstimate,
		ChargeState:     ChargeState(v.ChargeState),
		ChargeLimit:     v.ChargeLimit,
		ChargingRate:    v.ChargingRate,
		TimeToCharge:    v.ChargingTimeLeft,
		IsLocked:        v.IsLocked,
		IsOnline:        v.IsOnline,
		Odometer:        v.Odometer,
		CabinTemp:       v.CabinTemp,
		ExteriorTemp:    v.ExteriorTemp,
		Doors:           closuresFromRivian(v.Doors),
		Windows:         closuresFromRivian(v.Windows),
		Frunk:           ClosureStatus(v.Frunk),
		Liftgate:        ClosureStatus(v.Liftgate),
		TirePressures: TirePressures{
			FrontLeft:  v.TirePressures.FrontLeft,
			FrontRight: v.TirePressures.FrontRight,
			RearLeft:   v.TirePressures.RearLeft,
			RearRight:  v.TirePressures.RearRight,
			UpdatedAt:  v.TirePressures.UpdatedAt,
		},
	}

	// Location
	if v.Latitude != nil && v.Longitude != nil {
		state.Location = &Location{
			Latitude:  *v.Latitude,
			Longitude: *v.Longitude,
			UpdatedAt: v.UpdatedAt,
		}
	}

	// Tonneau cover (R1T only)
	if v.TonneauCover != nil {
		cs := ClosureStatus(*v.TonneauCover)
		state.TonneauCover = &cs
	}

	// Calculate derived metrics
	state.RangeStatus = DetermineRangeStatus(state.RangeEstimate)

	return state
}

// closuresFromRivian converts Rivian API ClosureState to our Closures.
func closuresFromRivian(rc rivian.ClosureState) Closures {
	return Closures{
		FrontLeft:  ClosureStatus(rc.FrontLeft),
		FrontRight: ClosureStatus(rc.FrontRight),
		RearLeft:   ClosureStatus(rc.RearLeft),
		RearRight:  ClosureStatus(rc.RearRight),
	}
}
