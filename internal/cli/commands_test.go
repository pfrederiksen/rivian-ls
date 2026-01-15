package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
	"github.com/pfrederiksen/rivian-ls/internal/store"
)

// mockClient implements rivian.Client for testing
type mockClient struct {
	state *rivian.VehicleState
	err   error
}

func (m *mockClient) Authenticate(ctx context.Context, email, password string) error {
	return nil
}

func (m *mockClient) SubmitOTP(ctx context.Context, otpCode string) error {
	return nil
}

func (m *mockClient) RefreshToken(ctx context.Context) error {
	return nil
}

func (m *mockClient) IsAuthenticated() bool {
	return true
}

func (m *mockClient) GetVehicles(ctx context.Context) ([]rivian.Vehicle, error) {
	return nil, nil
}

func (m *mockClient) GetVehicleState(ctx context.Context, vehicleID string) (*rivian.VehicleState, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.state, nil
}

func (m *mockClient) GetCredentials() *rivian.Credentials {
	return &rivian.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
}

func makeMockRivianState() *rivian.VehicleState {
	cabinTemp := 72.0
	chargingRate := 11.5

	return &rivian.VehicleState{
		VehicleID:       "vehicle-123",
		UpdatedAt:       time.Now(),
		BatteryLevel:    85.5,
		BatteryCapacity: 135.0,
		RangeEstimate:   250.0,
		ChargeState:     rivian.ChargeStateCharging,
		ChargeLimit:     80,
		ChargingRate:    &chargingRate,
		IsLocked:        true,
		IsOnline:        true,
		Odometer:        12345.6,
		CabinTemp:       &cabinTemp,
		Doors: rivian.ClosureState{
			FrontLeft:  rivian.ClosureStatusClosed,
			FrontRight: rivian.ClosureStatusClosed,
			RearLeft:   rivian.ClosureStatusClosed,
			RearRight:  rivian.ClosureStatusClosed,
		},
		Windows: rivian.ClosureState{
			FrontLeft:  rivian.ClosureStatusClosed,
			FrontRight: rivian.ClosureStatusClosed,
			RearLeft:   rivian.ClosureStatusClosed,
			RearRight:  rivian.ClosureStatusClosed,
		},
		Frunk:    rivian.ClosureStatusClosed,
		Liftgate: rivian.ClosureStatusClosed,
		TirePressures: rivian.TirePressures{
			FrontLeft:  42.0,
			FrontRight: 41.5,
			RearLeft:   42.0,
			RearRight:  41.5,
		},
	}
}

func TestStatusCommand_Run(t *testing.T) {
	tmpDir := t.TempDir()
	testStore, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = testStore.Close() }()

	mockState := makeMockRivianState()
	client := &mockClient{state: mockState}

	var buf bytes.Buffer
	cmd := NewStatusCommand(client, testStore, "vehicle-123", &buf)

	opts := StatusOptions{
		Format: FormatJSON,
		Pretty: false,
	}

	ctx := context.Background()
	err = cmd.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify output contains vehicle data
	output := buf.String()
	if !strings.Contains(output, "vehicle-123") {
		t.Error("Output missing vehicle ID")
	}
	if !strings.Contains(output, "85.5") {
		t.Error("Output missing battery level")
	}
}

func TestStatusCommand_Run_Offline(t *testing.T) {
	tmpDir := t.TempDir()
	testStore, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = testStore.Close() }()

	// Save a state to the store first
	ctx := context.Background()
	state := &model.VehicleState{
		VehicleID:     "vehicle-123",
		VIN:           "VIN123",
		Name:          "Test Vehicle",
		Model:         "R1T",
		UpdatedAt:     time.Now(),
		BatteryLevel:  80.0,
		RangeEstimate: 200.0,
		ChargeState:   model.ChargeStateNotCharging,
		RangeStatus:   model.RangeStatusNormal,
		IsOnline:      true,
		Doors: model.Closures{
			FrontLeft:  model.ClosureStatusClosed,
			FrontRight: model.ClosureStatusClosed,
			RearLeft:   model.ClosureStatusClosed,
			RearRight:  model.ClosureStatusClosed,
		},
		Windows: model.Closures{
			FrontLeft:  model.ClosureStatusClosed,
			FrontRight: model.ClosureStatusClosed,
			RearLeft:   model.ClosureStatusClosed,
			RearRight:  model.ClosureStatusClosed,
		},
		Frunk:         model.ClosureStatusClosed,
		Liftgate:      model.ClosureStatusClosed,
		TirePressures: model.TirePressures{UpdatedAt: time.Now()},
	}
	if err := testStore.SaveState(ctx, state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Client should not be called in offline mode
	client := &mockClient{err: fmt.Errorf("should not be called")}

	var buf bytes.Buffer
	cmd := NewStatusCommand(client, testStore, "vehicle-123", &buf)

	opts := StatusOptions{
		Format:  FormatText,
		Offline: true,
	}

	err = cmd.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify output contains cached data
	output := buf.String()
	if !strings.Contains(output, "Test Vehicle") {
		t.Error("Output missing vehicle name from cache")
	}
}

func TestExportCommand_Run(t *testing.T) {
	tmpDir := t.TempDir()
	testStore, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = testStore.Close() }()

	// Save some states
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 5; i++ {
		state := &model.VehicleState{
			VehicleID:     "vehicle-123",
			VIN:           "VIN123",
			Name:          "Test Vehicle",
			Model:         "R1T",
			UpdatedAt:     now.Add(time.Duration(i) * time.Hour),
			BatteryLevel:  float64(80 - i),
			RangeEstimate: 200.0,
			ChargeState:   model.ChargeStateNotCharging,
			RangeStatus:   model.RangeStatusNormal,
			IsOnline:      true,
			Doors: model.Closures{
				FrontLeft:  model.ClosureStatusClosed,
				FrontRight: model.ClosureStatusClosed,
				RearLeft:   model.ClosureStatusClosed,
				RearRight:  model.ClosureStatusClosed,
			},
			Windows: model.Closures{
				FrontLeft:  model.ClosureStatusClosed,
				FrontRight: model.ClosureStatusClosed,
				RearLeft:   model.ClosureStatusClosed,
				RearRight:  model.ClosureStatusClosed,
			},
			Frunk:         model.ClosureStatusClosed,
			Liftgate:      model.ClosureStatusClosed,
			TirePressures: model.TirePressures{UpdatedAt: now},
		}
		if err := testStore.SaveState(ctx, state); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}
	}

	var buf bytes.Buffer
	cmd := NewExportCommand(testStore, "vehicle-123", &buf)

	opts := ExportOptions{
		Format: FormatCSV,
		Since:  now,
		Limit:  10,
	}

	err = cmd.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify CSV output
	output := buf.String()
	if !strings.Contains(output, "BatteryLevel") {
		t.Error("CSV output missing header")
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Should have header + 5 data rows
	if len(lines) != 6 {
		t.Errorf("Expected 6 lines, got %d", len(lines))
	}
}

func TestExportCommand_Run_EmptyResults(t *testing.T) {
	tmpDir := t.TempDir()
	testStore, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = testStore.Close() }()

	var buf bytes.Buffer
	cmd := NewExportCommand(testStore, "vehicle-999", &buf)

	opts := ExportOptions{
		Format: FormatJSON,
		Since:  time.Now().Add(-24 * time.Hour),
	}

	ctx := context.Background()
	err = cmd.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should output "No states found"
	output := buf.String()
	if !strings.Contains(output, "No states found") {
		t.Error("Expected 'No states found' message")
	}
}

func TestExportCommand_Run_RangeQuery(t *testing.T) {
	tmpDir := t.TempDir()
	testStore, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = testStore.Close() }()

	// Save states spanning a time range
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 10; i++ {
		state := &model.VehicleState{
			VehicleID:     "vehicle-123",
			VIN:           "VIN123",
			Name:          "Test Vehicle",
			Model:         "R1T",
			UpdatedAt:     now.Add(time.Duration(i) * time.Hour),
			BatteryLevel:  float64(50 + i),
			RangeEstimate: 200.0,
			ChargeState:   model.ChargeStateNotCharging,
			RangeStatus:   model.RangeStatusNormal,
			IsOnline:      true,
			Doors: model.Closures{
				FrontLeft:  model.ClosureStatusClosed,
				FrontRight: model.ClosureStatusClosed,
				RearLeft:   model.ClosureStatusClosed,
				RearRight:  model.ClosureStatusClosed,
			},
			Windows: model.Closures{
				FrontLeft:  model.ClosureStatusClosed,
				FrontRight: model.ClosureStatusClosed,
				RearLeft:   model.ClosureStatusClosed,
				RearRight:  model.ClosureStatusClosed,
			},
			Frunk:         model.ClosureStatusClosed,
			Liftgate:      model.ClosureStatusClosed,
			TirePressures: model.TirePressures{UpdatedAt: now},
		}
		if err := testStore.SaveState(ctx, state); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}
	}

	var buf bytes.Buffer
	cmd := NewExportCommand(testStore, "vehicle-123", &buf)

	// Query middle range (hours 3-7)
	opts := ExportOptions{
		Format: FormatJSON,
		Since:  now.Add(3 * time.Hour),
		Until:  now.Add(7 * time.Hour),
		Pretty: false,
	}

	err = cmd.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify we got the right number of states
	output := buf.String()
	var states []*model.VehicleState
	if err := json.Unmarshal([]byte(output), &states); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if len(states) != 5 { // Hours 3, 4, 5, 6, 7
		t.Errorf("Expected 5 states, got %d", len(states))
	}
}
