package rivian

import (
	"context"
	"fmt"
	"time"
)

const (
	getVehiclesQuery = `
		query GetVehicles {
			currentUser {
				__typename
				vehicles {
					__typename
					id
					vin
					name
					vehicle {
						__typename
						model
					}
				}
			}
		}
	`

	getVehicleStateQuery = `
		query GetVehicleState($vehicleID: String!) {
			vehicleState(id: $vehicleID) {
				__typename
				gnssLocation {
					__typename
					latitude
					longitude
					timeStamp
				}
				batteryLevel {
					__typename
					timeStamp
					value
				}
				distanceToEmpty {
					__typename
					timeStamp
					value
				}
				chargerState {
					__typename
					timeStamp
					value
				}
				batteryLimit {
					__typename
					timeStamp
					value
				}
				timeToEndOfCharge {
					__typename
					timeStamp
					value
				}
				vehicleMileage {
					__typename
					timeStamp
					value
				}
				cabinClimateInteriorTemperature {
					__typename
					timeStamp
					value
				}
				doorFrontLeftLocked {
					__typename
					timeStamp
					value
				}
				doorFrontLeftClosed {
					__typename
					timeStamp
					value
				}
				doorFrontRightLocked {
					__typename
					timeStamp
					value
				}
				doorFrontRightClosed {
					__typename
					timeStamp
					value
				}
				doorRearLeftLocked {
					__typename
					timeStamp
					value
				}
				doorRearLeftClosed {
					__typename
					timeStamp
					value
				}
				doorRearRightLocked {
					__typename
					timeStamp
					value
				}
				doorRearRightClosed {
					__typename
					timeStamp
					value
				}
				windowFrontLeftClosed {
					__typename
					timeStamp
					value
				}
				windowFrontRightClosed {
					__typename
					timeStamp
					value
				}
				windowRearLeftClosed {
					__typename
					timeStamp
					value
				}
				windowRearRightClosed {
					__typename
					timeStamp
					value
				}
				closureFrunkLocked {
					__typename
					timeStamp
					value
				}
				closureFrunkClosed {
					__typename
					timeStamp
					value
				}
				closureLiftgateLocked {
					__typename
					timeStamp
					value
				}
				closureLiftgateClosed {
					__typename
					timeStamp
					value
				}
				closureTonneauLocked {
					__typename
					timeStamp
					value
				}
				closureTonneauClosed {
					__typename
					timeStamp
					value
				}
				tirePressureStatusFrontLeft {
					__typename
					timeStamp
					value
				}
				tirePressureStatusFrontRight {
					__typename
					timeStamp
					value
				}
				tirePressureStatusRearLeft {
					__typename
					timeStamp
					value
				}
				tirePressureStatusRearRight {
					__typename
					timeStamp
					value
				}
			}
		}
	`
)

// vehiclesResponse represents the response from GetVehicles query.
type vehiclesResponse struct {
	CurrentUser struct {
		Typename string `json:"__typename"`
		Vehicles []struct {
			Typename string `json:"__typename"`
			ID       string `json:"id"`
			VIN      string `json:"vin"`
			Name     string `json:"name"`
			Vehicle  struct {
				Typename string `json:"__typename"`
				Model    string `json:"model"`
			} `json:"vehicle"`
		} `json:"vehicles"`
	} `json:"currentUser"`
}

// timestampedValue represents a value with a timestamp from the Rivian API.
type timestampedValue[T any] struct {
	Typename  string `json:"__typename"`
	TimeStamp string `json:"timeStamp"` // API returns ISO 8601 string timestamps
	Value     T      `json:"value"`
}

// gnssLocation represents GPS location data.
type gnssLocation struct {
	Typename  string  `json:"__typename"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	TimeStamp string  `json:"timeStamp"` // API returns string timestamp for GNSS
}

// vehicleStateData represents the vehicle state data structure from the API.
type vehicleStateData struct {
	Typename                        string                        `json:"__typename"`
	GNSSLocation                    *gnssLocation                 `json:"gnssLocation"`
	BatteryLevel                    *timestampedValue[float64]    `json:"batteryLevel"`
	DistanceToEmpty                 *timestampedValue[float64]    `json:"distanceToEmpty"`
	ChargerState                    *timestampedValue[string]     `json:"chargerState"`
	BatteryLimit                    *timestampedValue[float64]    `json:"batteryLimit"`
	TimeToEndOfCharge               *timestampedValue[int64]      `json:"timeToEndOfCharge"`
	VehicleMileage                  *timestampedValue[float64]    `json:"vehicleMileage"`
	CabinClimateInteriorTemperature *timestampedValue[float64]    `json:"cabinClimateInteriorTemperature"`
	DoorFrontLeftLocked             *timestampedValue[string]     `json:"doorFrontLeftLocked"`
	DoorFrontLeftClosed             *timestampedValue[string]     `json:"doorFrontLeftClosed"`
	DoorFrontRightLocked            *timestampedValue[string]     `json:"doorFrontRightLocked"`
	DoorFrontRightClosed            *timestampedValue[string]     `json:"doorFrontRightClosed"`
	DoorRearLeftLocked              *timestampedValue[string]     `json:"doorRearLeftLocked"`
	DoorRearLeftClosed              *timestampedValue[string]     `json:"doorRearLeftClosed"`
	DoorRearRightLocked             *timestampedValue[string]     `json:"doorRearRightLocked"`
	DoorRearRightClosed             *timestampedValue[string]     `json:"doorRearRightClosed"`
	WindowFrontLeftClosed           *timestampedValue[string]     `json:"windowFrontLeftClosed"`
	WindowFrontRightClosed          *timestampedValue[string]     `json:"windowFrontRightClosed"`
	WindowRearLeftClosed            *timestampedValue[string]     `json:"windowRearLeftClosed"`
	WindowRearRightClosed           *timestampedValue[string]     `json:"windowRearRightClosed"`
	ClosureFrunkLocked              *timestampedValue[string]     `json:"closureFrunkLocked"`
	ClosureFrunkClosed              *timestampedValue[string]     `json:"closureFrunkClosed"`
	ClosureLiftgateLocked           *timestampedValue[string]     `json:"closureLiftgateLocked"`
	ClosureLiftgateClosed           *timestampedValue[string]     `json:"closureLiftgateClosed"`
	ClosureTonneauLocked            *timestampedValue[string]     `json:"closureTonneauLocked"`
	ClosureTonneauClosed            *timestampedValue[string]     `json:"closureTonneauClosed"`
	TirePressureStatusFrontLeft     *timestampedValue[string]     `json:"tirePressureStatusFrontLeft"`
	TirePressureStatusFrontRight    *timestampedValue[string]     `json:"tirePressureStatusFrontRight"`
	TirePressureStatusRearLeft      *timestampedValue[string]     `json:"tirePressureStatusRearLeft"`
	TirePressureStatusRearRight     *timestampedValue[string]     `json:"tirePressureStatusRearRight"`
}

// vehicleStateResponse represents the response from GetVehicleState query.
type vehicleStateResponse struct {
	VehicleState vehicleStateData `json:"vehicleState"`
}

// GetVehicles retrieves the list of vehicles for the authenticated user.
func (c *HTTPClient) GetVehicles(ctx context.Context) ([]Vehicle, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	var resp vehiclesResponse
	if err := c.doGraphQL(ctx, getVehiclesQuery, nil, &resp); err != nil {
		return nil, fmt.Errorf("get vehicles: %w", err)
	}

	vehicles := make([]Vehicle, len(resp.CurrentUser.Vehicles))
	for i, v := range resp.CurrentUser.Vehicles {
		vehicles[i] = Vehicle{
			ID:    v.ID,
			VIN:   v.VIN,
			Name:  v.Name,
			Model: v.Vehicle.Model,
			Year:  0, // modelYear not available in this query
		}
	}

	return vehicles, nil
}

// GetVehicleState retrieves the current state of a specific vehicle.
func (c *HTTPClient) GetVehicleState(ctx context.Context, vehicleID string) (*VehicleState, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	variables := map[string]interface{}{
		"vehicleID": vehicleID,
	}

	var resp vehicleStateResponse
	if err := c.doGraphQL(ctx, getVehicleStateQuery, variables, &resp); err != nil {
		return nil, fmt.Errorf("get vehicle state: %w", err)
	}

	return parseVehicleState(vehicleID, resp.VehicleState), nil
}

// parseVehicleState converts the API response to our domain model.
func parseVehicleState(vehicleID string, apiState vehicleStateData) *VehicleState {
	state := &VehicleState{
		VehicleID: vehicleID,
		UpdatedAt: time.Now(),
	}

	// Battery and charging
	if apiState.BatteryLevel != nil {
		state.BatteryLevel = apiState.BatteryLevel.Value
	}
	if apiState.DistanceToEmpty != nil {
		state.RangeEstimate = apiState.DistanceToEmpty.Value
	}
	if apiState.ChargerState != nil {
		state.ChargeState = parseChargeState(apiState.ChargerState.Value)
	}
	if apiState.BatteryLimit != nil {
		state.ChargeLimit = int(apiState.BatteryLimit.Value)
	}
	if apiState.TimeToEndOfCharge != nil && apiState.TimeToEndOfCharge.Value > 0 {
		// TimeToEndOfCharge.Value is in seconds until charge complete
		t := time.Now().Add(time.Duration(apiState.TimeToEndOfCharge.Value) * time.Second)
		state.ChargingTimeLeft = &t
	}

	// Vehicle state
	if apiState.VehicleMileage != nil {
		state.Odometer = apiState.VehicleMileage.Value
	}
	if apiState.CabinClimateInteriorTemperature != nil {
		temp := apiState.CabinClimateInteriorTemperature.Value
		state.CabinTemp = &temp
	}

	// Determine if locked (all doors locked)
	allLocked := true
	if apiState.DoorFrontLeftLocked != nil && apiState.DoorFrontLeftLocked.Value != "locked" {
		allLocked = false
	}
	if apiState.DoorFrontRightLocked != nil && apiState.DoorFrontRightLocked.Value != "locked" {
		allLocked = false
	}
	if apiState.DoorRearLeftLocked != nil && apiState.DoorRearLeftLocked.Value != "locked" {
		allLocked = false
	}
	if apiState.DoorRearRightLocked != nil && apiState.DoorRearRightLocked.Value != "locked" {
		allLocked = false
	}
	state.IsLocked = allLocked

	// Assume online if we have recent data
	state.IsOnline = true

	// Doors - combine locked and closed status
	state.Doors = ClosureState{
		FrontLeft:  parseDoorStatus(apiState.DoorFrontLeftClosed),
		FrontRight: parseDoorStatus(apiState.DoorFrontRightClosed),
		RearLeft:   parseDoorStatus(apiState.DoorRearLeftClosed),
		RearRight:  parseDoorStatus(apiState.DoorRearRightClosed),
	}

	// Windows
	state.Windows = ClosureState{
		FrontLeft:  parseClosureStatusFromTimestamped(apiState.WindowFrontLeftClosed),
		FrontRight: parseClosureStatusFromTimestamped(apiState.WindowFrontRightClosed),
		RearLeft:   parseClosureStatusFromTimestamped(apiState.WindowRearLeftClosed),
		RearRight:  parseClosureStatusFromTimestamped(apiState.WindowRearRightClosed),
	}

	// Frunk and liftgate
	state.Frunk = parseClosureStatusFromTimestamped(apiState.ClosureFrunkClosed)
	state.Liftgate = parseClosureStatusFromTimestamped(apiState.ClosureLiftgateClosed)

	// Tonneau cover
	if apiState.ClosureTonneauClosed != nil {
		cs := parseClosureStatusFromTimestamped(apiState.ClosureTonneauClosed)
		state.TonneauCover = &cs
	}

	// Tire pressures - API returns status strings, not pressure values
	// We'll just indicate if they're available
	if apiState.TirePressureStatusFrontLeft != nil {
		state.TirePressures.FrontLeft = 0 // Status only, no actual pressure
	}

	// Location
	if apiState.GNSSLocation != nil {
		lat := apiState.GNSSLocation.Latitude
		lon := apiState.GNSSLocation.Longitude
		state.Latitude = &lat
		state.Longitude = &lon
	}

	return state
}

// parseChargeState converts API charge status string to ChargeState enum.
func parseChargeState(status string) ChargeState {
	switch status {
	case "charging":
		return ChargeStateCharging
	case "complete", "fully_charged":
		return ChargeStateComplete
	case "scheduled":
		return ChargeStateScheduled
	case "disconnected", "not_connected":
		return ChargeStateDisconnected
	case "not_charging", "stopped":
		return ChargeStateNotCharging
	default:
		return ChargeStateUnknown
	}
}

// parseClosureState creates a ClosureState from individual closure statuses.
func parseClosureState(fl, fr, rl, rr *string) ClosureState {
	return ClosureState{
		FrontLeft:  parseClosureStatus(fl),
		FrontRight: parseClosureStatus(fr),
		RearLeft:   parseClosureStatus(rl),
		RearRight:  parseClosureStatus(rr),
	}
}

// parseClosureStatus converts API closure status string to ClosureStatus enum.
func parseClosureStatus(status *string) ClosureStatus {
	if status == nil {
		return ClosureStatusUnknown
	}
	switch *status {
	case "closed":
		return ClosureStatusClosed
	case "open":
		return ClosureStatusOpen
	default:
		return ClosureStatusUnknown
	}
}

// getFloat64 safely dereferences a float64 pointer.
func getFloat64(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

// parseClosureStatusFromTimestamped converts timestamped closure status to ClosureStatus.
func parseClosureStatusFromTimestamped(ts *timestampedValue[string]) ClosureStatus {
	if ts == nil {
		return ClosureStatusUnknown
	}
	return parseClosureStatus(&ts.Value)
}

// parseDoorStatus handles door status which may be "closed" or "open".
func parseDoorStatus(ts *timestampedValue[string]) ClosureStatus {
	if ts == nil {
		return ClosureStatusUnknown
	}
	return parseClosureStatus(&ts.Value)
}
