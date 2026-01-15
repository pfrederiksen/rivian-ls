package rivian

import (
	"context"
	"time"
)

// Client defines the interface for interacting with the Rivian API.
// This interface allows for easy mocking in tests.
type Client interface {
	// Authenticate performs login with email and password.
	// If MFA is enabled, it will return an error indicating OTP is required.
	Authenticate(ctx context.Context, email, password string) error

	// SubmitOTP submits a one-time password for MFA authentication.
	SubmitOTP(ctx context.Context, code string) error

	// RefreshToken refreshes the access token using the stored refresh token.
	RefreshToken(ctx context.Context) error

	// GetVehicles retrieves the list of vehicles for the authenticated user.
	GetVehicles(ctx context.Context) ([]Vehicle, error)

	// GetVehicleState retrieves the current state of a specific vehicle.
	GetVehicleState(ctx context.Context, vehicleID string) (*VehicleState, error)

	// IsAuthenticated returns true if the client has valid credentials.
	IsAuthenticated() bool
}

// Vehicle represents a Rivian vehicle.
type Vehicle struct {
	ID    string
	VIN   string
	Name  string
	Model string
	Year  int
}

// VehicleState represents the current state of a vehicle.
type VehicleState struct {
	VehicleID string
	UpdatedAt time.Time

	// Battery and charging
	BatteryLevel    float64 // Percentage (0-100)
	BatteryCapacity float64 // kWh
	RangeEstimate   float64 // Miles
	ChargeState     ChargeState
	ChargeLimit     int // Percentage (0-100)

	// Charging session info (only when charging)
	ChargingRate     *float64   // kW
	ChargingTimeLeft *time.Time // Estimated completion time

	// Vehicle state
	IsLocked     bool
	IsOnline     bool
	Odometer     float64 // Miles
	CabinTemp    *float64
	ExteriorTemp *float64

	// Closures
	Doors        ClosureState
	Windows      ClosureState
	Frunk        ClosureStatus
	Liftgate     ClosureStatus
	TonneauCover *ClosureStatus // R1T only

	// Tires
	TirePressures TirePressures

	// Location (if available)
	Latitude  *float64
	Longitude *float64
}

// ChargeState represents the vehicle's charging status.
type ChargeState string

const (
	ChargeStateUnknown      ChargeState = "unknown"
	ChargeStateNotCharging  ChargeState = "not_charging"
	ChargeStateCharging     ChargeState = "charging"
	ChargeStateComplete     ChargeState = "complete"
	ChargeStateScheduled    ChargeState = "scheduled"
	ChargeStateDisconnected ChargeState = "disconnected"
)

// ClosureState represents the state of doors, windows, etc.
type ClosureState struct {
	FrontLeft  ClosureStatus
	FrontRight ClosureStatus
	RearLeft   ClosureStatus
	RearRight  ClosureStatus
}

// ClosureStatus represents whether something is open or closed.
type ClosureStatus string

const (
	ClosureStatusClosed  ClosureStatus = "closed"
	ClosureStatusOpen    ClosureStatus = "open"
	ClosureStatusUnknown ClosureStatus = "unknown"
)

// TirePressures represents tire pressure readings for all four tires.
type TirePressures struct {
	FrontLeft  float64 // PSI
	FrontRight float64 // PSI
	RearLeft   float64 // PSI
	RearRight  float64 // PSI
	UpdatedAt  time.Time
}

// Credentials stores authentication credentials.
type Credentials struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	UserID       string
}

// OTPRequiredError is returned when OTP/MFA is required.
type OTPRequiredError struct {
	SessionID string
}

func (e *OTPRequiredError) Error() string {
	return "OTP/MFA code required for authentication"
}
