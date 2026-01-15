package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/model"
)

// saveTestStates is a helper to reduce duplicate test state creation
func saveTestStates(t *testing.T, store *Store, ctx context.Context, now time.Time, count int, timeDelta time.Duration, batteryFunc func(int) float64) {
	for i := 0; i < count; i++ {
		state := &model.VehicleState{
			VehicleID:     "vehicle-123",
			VIN:           "VIN123",
			Name:          "My R1T",
			Model:         "R1T",
			UpdatedAt:     now.Add(time.Duration(i) * timeDelta),
			BatteryLevel:  batteryFunc(i),
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

		if err := store.SaveState(ctx, state); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}
	}
}

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file not created")
	}

	// Verify schema was initialized
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='vehicle_states'").Scan(&count)
	if err != nil {
		t.Fatalf("Query schema failed: %v", err)
	}
	if count != 1 {
		t.Error("vehicle_states table not created")
	}
}

func TestSaveState(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	state := &model.VehicleState{
		VehicleID:       "vehicle-123",
		VIN:             "VIN123",
		Name:            "My R1T",
		Model:           "R1T",
		UpdatedAt:       now,
		BatteryLevel:    85.5,
		BatteryCapacity: 135.0,
		RangeEstimate:   250.0,
		RangeStatus:     model.RangeStatusNormal,
		ChargeState:     model.ChargeStateCharging,
		ChargeLimit:     80,
		IsLocked:        true,
		IsOnline:        true,
		Odometer:        12345.6,
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
		Frunk:    model.ClosureStatusClosed,
		Liftgate: model.ClosureStatusClosed,
		TirePressures: model.TirePressures{
			FrontLeft:  42.0,
			FrontRight: 41.5,
			RearLeft:   42.0,
			RearRight:  41.5,
			UpdatedAt:  now,
		},
		Location: &model.Location{
			Latitude:  37.7749,
			Longitude: -122.4194,
		},
	}

	// Save state
	err = store.SaveState(ctx, state)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Verify state was saved
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM vehicle_states WHERE vehicle_id = ?", "vehicle-123").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 state, got %d", count)
	}
}

func TestGetLatestState(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Save multiple states
	saveTestStates(t, store, ctx, now, 3, time.Minute, func(i int) float64 { return float64(80 + i) })

	// Get latest state
	latest, err := store.GetLatestState(ctx, "vehicle-123")
	if err != nil {
		t.Fatalf("GetLatestState failed: %v", err)
	}

	if latest == nil {
		t.Fatal("GetLatestState returned nil")
	}

	// Should be the last state (battery level 82)
	if latest.BatteryLevel != 82.0 {
		t.Errorf("Expected battery level 82, got %v", latest.BatteryLevel)
	}

	// Verify all fields were restored
	if latest.VehicleID != "vehicle-123" {
		t.Errorf("VehicleID mismatch: got %s", latest.VehicleID)
	}
	if latest.VIN != "VIN123" {
		t.Errorf("VIN mismatch: got %s", latest.VIN)
	}
	if latest.Doors.FrontLeft != model.ClosureStatusClosed {
		t.Error("Doors not restored correctly")
	}
}

func TestGetLatestState_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Get state for non-existent vehicle
	state, err := store.GetLatestState(ctx, "non-existent")
	if err != nil {
		t.Fatalf("GetLatestState failed: %v", err)
	}

	if state != nil {
		t.Error("Expected nil for non-existent vehicle")
	}
}

func TestGetStateHistory(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Save states over a time range
	saveTestStates(t, store, ctx, now, 10, time.Hour, func(i int) float64 { return float64(50 + i) })

	// Get history since 5 hours ago, limit 3
	since := now.Add(5 * time.Hour)
	history, err := store.GetStateHistory(ctx, "vehicle-123", since, 3)
	if err != nil {
		t.Fatalf("GetStateHistory failed: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 states, got %d", len(history))
	}

	// Should be in reverse chronological order (newest first)
	if history[0].BatteryLevel != 59.0 { // Hour 9
		t.Errorf("Expected battery 59, got %v", history[0].BatteryLevel)
	}
}

func TestGetStates(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Save states
	saveTestStates(t, store, ctx, now, 10, time.Hour, func(i int) float64 { return float64(50 + i) })

	// Get states between hours 3 and 7
	start := now.Add(3 * time.Hour)
	end := now.Add(7 * time.Hour)
	states, err := store.GetStates(ctx, "vehicle-123", start, end)
	if err != nil {
		t.Fatalf("GetStates failed: %v", err)
	}

	if len(states) != 5 { // Hours 3, 4, 5, 6, 7
		t.Errorf("Expected 5 states, got %d", len(states))
	}
}

func TestDeleteOldStates(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Save old and new states
	for i := 0; i < 10; i++ {
		state := &model.VehicleState{
			VehicleID:     "vehicle-123",
			VIN:           "VIN123",
			Name:          "My R1T",
			Model:         "R1T",
			UpdatedAt:     now.Add(time.Duration(i-5) * time.Hour), // Some before now, some after
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

		if err := store.SaveState(ctx, state); err != nil {
			t.Fatalf("SaveState failed: %v", err)
		}
	}

	// Delete states older than now
	deleted, err := store.DeleteOldStates(ctx, now)
	if err != nil {
		t.Fatalf("DeleteOldStates failed: %v", err)
	}

	if deleted != 5 {
		t.Errorf("Expected 5 deleted, got %d", deleted)
	}

	// Verify remaining states
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM vehicle_states").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 remaining states, got %d", count)
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Save states for multiple vehicles
	vehicles := []string{"vehicle-1", "vehicle-2"}
	for _, vehicleID := range vehicles {
		for i := 0; i < 5; i++ {
			state := &model.VehicleState{
				VehicleID:     vehicleID,
				VIN:           "VIN" + vehicleID,
				Name:          "Vehicle",
				Model:         "R1T",
				UpdatedAt:     now.Add(time.Duration(i) * time.Hour),
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
				TirePressures: model.TirePressures{UpdatedAt: now},
			}

			if err := store.SaveState(ctx, state); err != nil {
				t.Fatalf("SaveState failed: %v", err)
			}
		}
	}

	// Get stats
	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalStates != 10 {
		t.Errorf("Expected 10 total states, got %d", stats.TotalStates)
	}

	if stats.UniqueVehicles != 2 {
		t.Errorf("Expected 2 unique vehicles, got %d", stats.UniqueVehicles)
	}

	if stats.OldestState == nil {
		t.Error("OldestState is nil")
	}

	if stats.NewestState == nil {
		t.Error("NewestState is nil")
	}

	if stats.DatabaseSize == 0 {
		t.Error("DatabaseSize is 0")
	}
}
