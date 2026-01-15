package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pfrederiksen/rivian-ls/internal/model"
)

// Store manages local persistence of vehicle state snapshots
type Store struct {
	db *sql.DB
}

// NewStore creates a new store at the given database path
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable Write-Ahead Logging for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	store := &Store{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// initSchema creates the database schema
func (s *Store) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS vehicle_states (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vehicle_id TEXT NOT NULL,
			vin TEXT,
			name TEXT,
			model TEXT,
			timestamp DATETIME NOT NULL,
			battery_level REAL,
			battery_capacity REAL,
			range_estimate REAL,
			range_status TEXT,
			charge_state TEXT,
			charge_limit INTEGER,
			charging_rate REAL,
			time_to_charge DATETIME,
			is_locked BOOLEAN,
			is_online BOOLEAN,
			odometer REAL,
			cabin_temp REAL,
			exterior_temp REAL,
			latitude REAL,
			longitude REAL,
			doors_json TEXT,
			windows_json TEXT,
			frunk TEXT,
			liftgate TEXT,
			tonneau_cover TEXT,
			tire_pressures_json TEXT,
			ready_score REAL,
			state_json TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_vehicle_states_vehicle_id
			ON vehicle_states(vehicle_id);

		CREATE INDEX IF NOT EXISTS idx_vehicle_states_timestamp
			ON vehicle_states(timestamp DESC);

		CREATE INDEX IF NOT EXISTS idx_vehicle_states_vehicle_timestamp
			ON vehicle_states(vehicle_id, timestamp DESC);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveState stores a vehicle state snapshot
func (s *Store) SaveState(ctx context.Context, state *model.VehicleState) error {
	if state == nil {
		return fmt.Errorf("state is nil")
	}

	// Serialize complex fields to JSON
	doorsJSON, err := json.Marshal(state.Doors)
	if err != nil {
		return fmt.Errorf("marshal doors: %w", err)
	}

	windowsJSON, err := json.Marshal(state.Windows)
	if err != nil {
		return fmt.Errorf("marshal windows: %w", err)
	}

	tireJSON, err := json.Marshal(state.TirePressures)
	if err != nil {
		return fmt.Errorf("marshal tire pressures: %w", err)
	}

	// Store full state as JSON for future compatibility
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Prepare nullable fields
	var latitude, longitude *float64
	if state.Location != nil {
		latitude = &state.Location.Latitude
		longitude = &state.Location.Longitude
	}

	var tonneauCover *string
	if state.TonneauCover != nil {
		tc := string(*state.TonneauCover)
		tonneauCover = &tc
	}

	var timeToCharge *time.Time
	if state.TimeToCharge != nil {
		timeToCharge = state.TimeToCharge
	}

	query := `
		INSERT INTO vehicle_states (
			vehicle_id, vin, name, model, timestamp,
			battery_level, battery_capacity, range_estimate, range_status,
			charge_state, charge_limit, charging_rate, time_to_charge,
			is_locked, is_online, odometer,
			cabin_temp, exterior_temp,
			latitude, longitude,
			doors_json, windows_json, frunk, liftgate, tonneau_cover,
			tire_pressures_json, ready_score, state_json
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?,
			?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?
		)
	`

	_, err = s.db.ExecContext(ctx, query,
		state.VehicleID, state.VIN, state.Name, state.Model, state.UpdatedAt,
		state.BatteryLevel, state.BatteryCapacity, state.RangeEstimate, state.RangeStatus,
		state.ChargeState, state.ChargeLimit, state.ChargingRate, timeToCharge,
		state.IsLocked, state.IsOnline, state.Odometer,
		state.CabinTemp, state.ExteriorTemp,
		latitude, longitude,
		string(doorsJSON), string(windowsJSON), state.Frunk, state.Liftgate, tonneauCover,
		string(tireJSON), state.ReadyScore, string(stateJSON),
	)

	return err
}

// GetLatestState retrieves the most recent state for a vehicle
func (s *Store) GetLatestState(ctx context.Context, vehicleID string) (*model.VehicleState, error) {
	query := `
		SELECT state_json
		FROM vehicle_states
		WHERE vehicle_id = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var stateJSON string
	err := s.db.QueryRowContext(ctx, query, vehicleID).Scan(&stateJSON)
	if err == sql.ErrNoRows {
		return nil, nil // No state found
	}
	if err != nil {
		return nil, fmt.Errorf("query state: %w", err)
	}

	var state model.VehicleState
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	return &state, nil
}

// GetStateHistory retrieves historical states for a vehicle
func (s *Store) GetStateHistory(ctx context.Context, vehicleID string, since time.Time, limit int) ([]*model.VehicleState, error) {
	query := `
		SELECT state_json
		FROM vehicle_states
		WHERE vehicle_id = ? AND timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, vehicleID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var states []*model.VehicleState
	for rows.Next() {
		var stateJSON string
		if err := rows.Scan(&stateJSON); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		var state model.VehicleState
		if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
			return nil, fmt.Errorf("unmarshal state: %w", err)
		}

		states = append(states, &state)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return states, nil
}

// GetStates retrieves states within a time range
func (s *Store) GetStates(ctx context.Context, vehicleID string, start, end time.Time) ([]*model.VehicleState, error) {
	query := `
		SELECT state_json
		FROM vehicle_states
		WHERE vehicle_id = ? AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC
	`

	rows, err := s.db.QueryContext(ctx, query, vehicleID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query states: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var states []*model.VehicleState
	for rows.Next() {
		var stateJSON string
		if err := rows.Scan(&stateJSON); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		var state model.VehicleState
		if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
			return nil, fmt.Errorf("unmarshal state: %w", err)
		}

		states = append(states, &state)
	}

	return states, rows.Err()
}

// DeleteOldStates removes states older than the given time
func (s *Store) DeleteOldStates(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM vehicle_states
		WHERE timestamp < ?
	`, before)

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetStats returns storage statistics
func (s *Store) GetStats(ctx context.Context) (*StoreStats, error) {
	var stats StoreStats

	// Count total states
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM vehicle_states
	`).Scan(&stats.TotalStates)
	if err != nil {
		return nil, err
	}

	// Count unique vehicles
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT vehicle_id) FROM vehicle_states
	`).Scan(&stats.UniqueVehicles)
	if err != nil {
		return nil, err
	}

	// Get oldest and newest timestamps
	var oldestStr, newestStr *string
	err = s.db.QueryRowContext(ctx, `
		SELECT MIN(timestamp), MAX(timestamp)
		FROM vehicle_states
	`).Scan(&oldestStr, &newestStr)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Parse timestamp strings
	if oldestStr != nil {
		t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", *oldestStr)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", *oldestStr)
			if err != nil {
				return nil, fmt.Errorf("parse oldest timestamp: %w", err)
			}
		}
		stats.OldestState = &t
	}

	if newestStr != nil {
		t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", *newestStr)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", *newestStr)
			if err != nil {
				return nil, fmt.Errorf("parse newest timestamp: %w", err)
			}
		}
		stats.NewestState = &t
	}

	// Get database size (SQLite-specific)
	err = s.db.QueryRowContext(ctx, `
		SELECT page_count * page_size as size FROM pragma_page_count(), pragma_page_size()
	`).Scan(&stats.DatabaseSize)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// StoreStats contains storage statistics
type StoreStats struct {
	TotalStates    int64
	UniqueVehicles int64
	OldestState    *time.Time
	NewestState    *time.Time
	DatabaseSize   int64 // bytes
}
