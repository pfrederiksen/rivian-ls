package rivian

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetVehicles_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify u-sess header (userSessionToken)
		uSess := r.Header.Get("u-sess")
		if uSess != "test-token" {
			t.Errorf("Expected u-sess header 'test-token', got %s", uSess)
		}

		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if !strings.Contains(req.Query, "query GetVehicles") {
			t.Error("Expected GetVehicles query")
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"currentUser": map[string]interface{}{
					"__typename": "User",
					"vehicles": []map[string]interface{}{
						{
							"__typename": "UserVehicle",
							"id":         "vehicle-1",
							"vin":        "VIN123",
							"name":       "My R1T",
							"vehicle": map[string]interface{}{
								"__typename": "Vehicle",
								"model":      "R1T",
							},
						},
						{
							"__typename": "UserVehicle",
							"id":         "vehicle-2",
							"vin":        "VIN456",
							"name":       "My R1S",
							"vehicle": map[string]interface{}{
								"__typename": "Vehicle",
								"model":      "R1S",
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(
		WithBaseURL(server.URL),
		WithCredentials(&Credentials{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}),
	)

	vehicles, err := client.GetVehicles(context.Background())
	if err != nil {
		t.Fatalf("GetVehicles failed: %v", err)
	}

	if len(vehicles) != 2 {
		t.Fatalf("Expected 2 vehicles, got %d", len(vehicles))
	}

	// Check first vehicle
	if vehicles[0].ID != "vehicle-1" {
		t.Errorf("Expected ID vehicle-1, got %s", vehicles[0].ID)
	}
	if vehicles[0].VIN != "VIN123" {
		t.Errorf("Expected VIN VIN123, got %s", vehicles[0].VIN)
	}
	if vehicles[0].Name != "My R1T" {
		t.Errorf("Expected name 'My R1T', got %s", vehicles[0].Name)
	}
	if vehicles[0].Model != "R1T" {
		t.Errorf("Expected model R1T, got %s", vehicles[0].Model)
	}
}

func TestGetVehicles_NotAuthenticated(t *testing.T) {
	client := NewHTTPClient()
	_, err := client.GetVehicles(context.Background())

	if err == nil {
		t.Fatal("Expected error when not authenticated")
	}

	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("Expected 'not authenticated' error, got: %v", err)
	}
}

func TestGetVehicleState_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if !strings.Contains(req.Query, "query GetVehicleState") {
			t.Error("Expected GetVehicleState query")
		}

		// Verify vehicleID variable (uppercase ID)
		if vehicleID, ok := req.Variables["vehicleID"].(string); !ok || vehicleID != "vehicle-1" {
			t.Errorf("Expected vehicleID variable 'vehicle-1', got %v", req.Variables["vehicleID"])
		}

		timestamp := "2024-01-15T10:30:00.000Z"

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"vehicleState": map[string]interface{}{
					"__typename": "VehicleState",
					"gnssLocation": map[string]interface{}{
						"__typename": "GNSSLocation",
						"latitude":   37.7749,
						"longitude":  -122.4194,
						"timeStamp":  timestamp,
					},
					"batteryLevel": map[string]interface{}{
						"__typename": "BatteryLevel",
						"timeStamp":  timestamp,
						"value":      85.5,
					},
					"distanceToEmpty": map[string]interface{}{
						"__typename": "DistanceToEmpty",
						"timeStamp":  timestamp,
						"value":      250.0,
					},
					"chargerState": map[string]interface{}{
						"__typename": "ChargerState",
						"timeStamp":  timestamp,
						"value":      "charging",
					},
					"batteryLimit": map[string]interface{}{
						"__typename": "BatteryLimit",
						"timeStamp":  timestamp,
						"value":      80.0,
					},
					"vehicleMileage": map[string]interface{}{
						"__typename": "VehicleMileage",
						"timeStamp":  timestamp,
						"value":      12345.6,
					},
					"cabinClimateInteriorTemperature": map[string]interface{}{
						"__typename": "CabinClimateInteriorTemperature",
						"timeStamp":  timestamp,
						"value":      72.0,
					},
					"doorFrontLeftLocked": map[string]interface{}{
						"__typename": "DoorStatus",
						"timeStamp":  timestamp,
						"value":      "locked",
					},
					"doorFrontLeftClosed": map[string]interface{}{
						"__typename": "DoorStatus",
						"timeStamp":  timestamp,
						"value":      "closed",
					},
					"doorFrontRightLocked": map[string]interface{}{
						"__typename": "DoorStatus",
						"timeStamp":  timestamp,
						"value":      "locked",
					},
					"doorFrontRightClosed": map[string]interface{}{
						"__typename": "DoorStatus",
						"timeStamp":  timestamp,
						"value":      "closed",
					},
					"doorRearLeftLocked": map[string]interface{}{
						"__typename": "DoorStatus",
						"timeStamp":  timestamp,
						"value":      "locked",
					},
					"doorRearLeftClosed": map[string]interface{}{
						"__typename": "DoorStatus",
						"timeStamp":  timestamp,
						"value":      "closed",
					},
					"doorRearRightLocked": map[string]interface{}{
						"__typename": "DoorStatus",
						"timeStamp":  timestamp,
						"value":      "locked",
					},
					"doorRearRightClosed": map[string]interface{}{
						"__typename": "DoorStatus",
						"timeStamp":  timestamp,
						"value":      "closed",
					},
					"closureFrunkClosed": map[string]interface{}{
						"__typename": "ClosureStatus",
						"timeStamp":  timestamp,
						"value":      "closed",
					},
					"tirePressureStatusFrontLeft": map[string]interface{}{
						"__typename": "TirePressureStatus",
						"timeStamp":  timestamp,
						"value":      "normal",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(
		WithBaseURL(server.URL),
		WithCredentials(&Credentials{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}),
	)

	state, err := client.GetVehicleState(context.Background(), "vehicle-1")
	if err != nil {
		t.Fatalf("GetVehicleState failed: %v", err)
	}

	if state.VehicleID != "vehicle-1" {
		t.Errorf("Expected vehicle ID vehicle-1, got %s", state.VehicleID)
	}
	if state.BatteryLevel != 85.5 {
		t.Errorf("Expected battery level 85.5, got %f", state.BatteryLevel)
	}
	if state.RangeEstimate != 250.0 {
		t.Errorf("Expected range estimate 250.0, got %f", state.RangeEstimate)
	}
	if state.ChargeState != ChargeStateCharging {
		t.Errorf("Expected charge state charging, got %s", state.ChargeState)
	}
	if !state.IsLocked {
		t.Error("Expected vehicle to be locked")
	}
	if !state.IsOnline {
		t.Error("Expected vehicle to be online")
	}
	if state.Odometer != 12345.6 {
		t.Errorf("Expected odometer 12345.6, got %f", state.Odometer)
	}
	if state.CabinTemp == nil || *state.CabinTemp != 72.0 {
		t.Errorf("Expected cabin temp 72.0, got %v", state.CabinTemp)
	}
	if state.Doors.FrontLeft != ClosureStatusClosed {
		t.Errorf("Expected front left door closed, got %s", state.Doors.FrontLeft)
	}
	if state.Frunk != ClosureStatusClosed {
		t.Errorf("Expected frunk closed, got %s", state.Frunk)
	}
	if state.Latitude == nil || *state.Latitude != 37.7749 {
		t.Errorf("Expected latitude 37.7749, got %v", state.Latitude)
	}
	if state.Longitude == nil || *state.Longitude != -122.4194 {
		t.Errorf("Expected longitude -122.4194, got %v", state.Longitude)
	}
}

func TestParseChargeState(t *testing.T) {
	tests := []struct {
		input string
		want  ChargeState
	}{
		{"charging", ChargeStateCharging},
		{"complete", ChargeStateComplete},
		{"fully_charged", ChargeStateComplete},
		{"charging_complete", ChargeStateComplete},
		{"scheduled", ChargeStateScheduled},
		{"disconnected", ChargeStateDisconnected},
		{"not_connected", ChargeStateDisconnected},
		{"not_charging", ChargeStateNotCharging},
		{"stopped", ChargeStateNotCharging},
		{"unknown_status", ChargeStateUnknown},
		{"", ChargeStateDisconnected},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseChargeState(tt.input)
			if got != tt.want {
				t.Errorf("parseChargeState(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseClosureStatus(t *testing.T) {
	closed := "closed"
	open := "open"
	unknown := "unknown"

	tests := []struct {
		name  string
		input *string
		want  ClosureStatus
	}{
		{"nil", nil, ClosureStatusUnknown},
		{"closed", &closed, ClosureStatusClosed},
		{"open", &open, ClosureStatusOpen},
		{"unknown", &unknown, ClosureStatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseClosureStatus(tt.input)
			if got != tt.want {
				t.Errorf("parseClosureStatus(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
