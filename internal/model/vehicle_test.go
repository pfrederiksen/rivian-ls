package model

import (
	"testing"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

func TestClosures_AllClosed(t *testing.T) {
	tests := []struct {
		name     string
		closures Closures
		want     bool
	}{
		{
			name: "all closed",
			closures: Closures{
				FrontLeft:  ClosureStatusClosed,
				FrontRight: ClosureStatusClosed,
				RearLeft:   ClosureStatusClosed,
				RearRight:  ClosureStatusClosed,
			},
			want: true,
		},
		{
			name: "one open",
			closures: Closures{
				FrontLeft:  ClosureStatusOpen,
				FrontRight: ClosureStatusClosed,
				RearLeft:   ClosureStatusClosed,
				RearRight:  ClosureStatusClosed,
			},
			want: false,
		},
		{
			name: "one unknown",
			closures: Closures{
				FrontLeft:  ClosureStatusUnknown,
				FrontRight: ClosureStatusClosed,
				RearLeft:   ClosureStatusClosed,
				RearRight:  ClosureStatusClosed,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.closures.AllClosed(); got != tt.want {
				t.Errorf("AllClosed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClosures_AnyOpen(t *testing.T) {
	tests := []struct {
		name     string
		closures Closures
		want     bool
	}{
		{
			name: "all closed",
			closures: Closures{
				FrontLeft:  ClosureStatusClosed,
				FrontRight: ClosureStatusClosed,
				RearLeft:   ClosureStatusClosed,
				RearRight:  ClosureStatusClosed,
			},
			want: false,
		},
		{
			name: "one open",
			closures: Closures{
				FrontLeft:  ClosureStatusOpen,
				FrontRight: ClosureStatusClosed,
				RearLeft:   ClosureStatusClosed,
				RearRight:  ClosureStatusClosed,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.closures.AnyOpen(); got != tt.want {
				t.Errorf("AnyOpen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTirePressures_AnyLow(t *testing.T) {
	tests := []struct {
		name      string
		pressures TirePressures
		threshold float64
		want      bool
	}{
		{
			name: "all normal",
			pressures: TirePressures{
				FrontLeft:  35.0,
				FrontRight: 35.0,
				RearLeft:   35.0,
				RearRight:  35.0,
			},
			threshold: 30.0,
			want:      false,
		},
		{
			name: "one low",
			pressures: TirePressures{
				FrontLeft:  25.0,
				FrontRight: 35.0,
				RearLeft:   35.0,
				RearRight:  35.0,
			},
			threshold: 30.0,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pressures.AnyLow(tt.threshold); got != tt.want {
				t.Errorf("AnyLow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineRangeStatus(t *testing.T) {
	tests := []struct {
		name  string
		miles float64
		want  RangeStatus
	}{
		{"critical", 20.0, RangeStatusCritical},
		{"low", 40.0, RangeStatusLow},
		{"normal", 100.0, RangeStatusNormal},
		{"high", 300.0, RangeStatusNormal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetermineRangeStatus(tt.miles); got != tt.want {
				t.Errorf("DetermineRangeStatus(%v) = %v, want %v", tt.miles, got, tt.want)
			}
		})
	}
}

func TestFromRivianVehicle(t *testing.T) {
	now := time.Now()
	riv := rivian.Vehicle{
		ID:    "vehicle-123",
		VIN:   "VIN123456",
		Name:  "My R1T",
		Model: "R1T",
		Year:  2023,
	}

	state := FromRivianVehicle(riv)

	if state.VehicleID != riv.ID {
		t.Errorf("VehicleID = %v, want %v", state.VehicleID, riv.ID)
	}
	if state.VIN != riv.VIN {
		t.Errorf("VIN = %v, want %v", state.VIN, riv.VIN)
	}
	if state.Name != riv.Name {
		t.Errorf("Name = %v, want %v", state.Name, riv.Name)
	}
	if state.Model != riv.Model {
		t.Errorf("Model = %v, want %v", state.Model, riv.Model)
	}
	if state.UpdatedAt.Before(now) {
		t.Error("UpdatedAt should be recent")
	}
}

func TestFromRivianVehicleState(t *testing.T) {
	lat := 37.7749
	lon := -122.4194
	cabinTemp := 72.0
	chargingRate := 11.5

	rivState := &rivian.VehicleState{
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
		Latitude:  &lat,
		Longitude: &lon,
	}

	state := FromRivianVehicleState(rivState)

	if state.VehicleID != rivState.VehicleID {
		t.Errorf("VehicleID = %v, want %v", state.VehicleID, rivState.VehicleID)
	}
	if state.BatteryLevel != rivState.BatteryLevel {
		t.Errorf("BatteryLevel = %v, want %v", state.BatteryLevel, rivState.BatteryLevel)
	}
	if state.RangeEstimate != rivState.RangeEstimate {
		t.Errorf("RangeEstimate = %v, want %v", state.RangeEstimate, rivState.RangeEstimate)
	}
	if state.ChargeState != ChargeStateCharging {
		t.Errorf("ChargeState = %v, want %v", state.ChargeState, ChargeStateCharging)
	}
	if !state.IsLocked {
		t.Error("Expected IsLocked = true")
	}
	if !state.IsOnline {
		t.Error("Expected IsOnline = true")
	}
	if state.Location == nil {
		t.Fatal("Expected Location to be set")
	}
	if state.Location.Latitude != lat {
		t.Errorf("Latitude = %v, want %v", state.Location.Latitude, lat)
	}
	if state.Location.Longitude != lon {
		t.Errorf("Longitude = %v, want %v", state.Location.Longitude, lon)
	}
	if state.RangeStatus != RangeStatusNormal {
		t.Errorf("RangeStatus = %v, want %v", state.RangeStatus, RangeStatusNormal)
	}
}
