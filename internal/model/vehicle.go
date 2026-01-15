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

// TirePressureStatus represents the status of a tire's pressure.
type TirePressureStatus string

const (
	TirePressureStatusUnknown TirePressureStatus = "unknown"
	TirePressureStatusOK      TirePressureStatus = "OK"
	TirePressureStatusLow     TirePressureStatus = "low"
	TirePressureStatusHigh    TirePressureStatus = "high"
)

// TirePressures represents tire pressure readings.
type TirePressures struct {
	FrontLeft  float64 // PSI (0 if not available)
	FrontRight float64 // PSI (0 if not available)
	RearLeft   float64 // PSI (0 if not available)
	RearRight  float64 // PSI (0 if not available)

	// Status from API (API doesn't provide PSI, only status)
	FrontLeftStatus  TirePressureStatus
	FrontRightStatus TirePressureStatus
	RearLeftStatus   TirePressureStatus
	RearRightStatus  TirePressureStatus

	UpdatedAt  time.Time
}

// AnyLow returns true if any tire is below the threshold (typically 30 PSI).
func (t TirePressures) AnyLow(threshold float64) bool {
	return t.FrontLeft < threshold ||
		t.FrontRight < threshold ||
		t.RearLeft < threshold ||
		t.RearRight < threshold
}

// AnyStatusLow returns true if any tire has a "low" status.
func (t TirePressures) AnyStatusLow() bool {
	return t.FrontLeftStatus == TirePressureStatusLow ||
		t.FrontRightStatus == TirePressureStatusLow ||
		t.RearLeftStatus == TirePressureStatusLow ||
		t.RearRightStatus == TirePressureStatusLow
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
		RangeEstimate:   kilometersToMiles(v.RangeEstimate),
		ChargeState:     ChargeState(v.ChargeState),
		ChargeLimit:     v.ChargeLimit,
		ChargingRate:    v.ChargingRate,
		TimeToCharge:    v.ChargingTimeLeft,
		IsLocked:        v.IsLocked,
		IsOnline:        v.IsOnline,
		Odometer:        metersToMiles(v.Odometer),
		CabinTemp:       celsiusToFahrenheit(v.CabinTemp),
		ExteriorTemp:    celsiusToFahrenheit(v.ExteriorTemp),
		Doors:           closuresFromRivian(v.Doors),
		Windows:         closuresFromRivian(v.Windows),
		Frunk:           ClosureStatus(v.Frunk),
		Liftgate:        ClosureStatus(v.Liftgate),
		TirePressures: TirePressures{
			FrontLeft:        v.TirePressures.FrontLeft,
			FrontRight:       v.TirePressures.FrontRight,
			RearLeft:         v.TirePressures.RearLeft,
			RearRight:        v.TirePressures.RearRight,
			FrontLeftStatus:  TirePressureStatus(v.TirePressures.FrontLeftStatus),
			FrontRightStatus: TirePressureStatus(v.TirePressures.FrontRightStatus),
			RearLeftStatus:   TirePressureStatus(v.TirePressures.RearLeftStatus),
			RearRightStatus:  TirePressureStatus(v.TirePressures.RearRightStatus),
			UpdatedAt:        v.TirePressures.UpdatedAt,
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

	// Estimate battery capacity if not provided by API
	if state.BatteryCapacity == 0 && state.BatteryLevel > 0 && state.RangeEstimate > 0 {
		state.BatteryCapacity = estimateBatteryCapacity(state.Model, state.BatteryLevel, state.RangeEstimate)
	}

	return state
}

// estimateBatteryCapacity calculates battery capacity from current charge level and range.
// Uses typical efficiency values for Rivian vehicles to estimate total capacity.
func estimateBatteryCapacity(model string, batteryPercent, rangeMiles float64) float64 {
	if batteryPercent <= 0 || rangeMiles <= 0 {
		return 0
	}

	// Calculate range at 100%
	rangeAt100 := rangeMiles / (batteryPercent / 100.0)

	// Typical efficiency for Rivian vehicles (mi/kWh)
	// R1T: ~2.0 mi/kWh average
	// R1S: ~2.1 mi/kWh average (slightly more efficient due to aerodynamics)
	var efficiency float64
	if model == "R1S" {
		efficiency = 2.1
	} else {
		efficiency = 2.0 // R1T or unknown
	}

	// Calculate capacity: range / efficiency
	// This gives us the usable capacity
	capacity := rangeAt100 / efficiency

	return capacity
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

// metersToMiles converts meters to miles.
// Rivian API returns odometer in meters, we display in miles.
func metersToMiles(meters float64) float64 {
	return meters / 1609.34
}

// kilometersToMiles converts kilometers to miles.
// Rivian API returns range estimate in kilometers, we display in miles.
func kilometersToMiles(km float64) float64 {
	return km / 1.60934
}

// celsiusToFahrenheit converts temperature from Celsius to Fahrenheit.
// Rivian API returns temperatures in Celsius, we display in Fahrenheit.
func celsiusToFahrenheit(celsius *float64) *float64 {
	if celsius == nil {
		return nil
	}
	fahrenheit := (*celsius * 9.0 / 5.0) + 32.0
	return &fahrenheit
}
