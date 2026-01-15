package model

import (
	"testing"
	"time"
)

func TestCalculateReadyScore(t *testing.T) {
	tests := []struct {
		name  string
		state *VehicleState
		want  *float64
	}{
		{
			name: "perfect score",
			state: &VehicleState{
				IsOnline:      true,
				BatteryLevel:  100,
				RangeEstimate: 300,
				Doors:         Closures{ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				Windows:       Closures{ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				Frunk:         ClosureStatusClosed,
				Liftgate:      ClosureStatusClosed,
				IsLocked:      true,
				TirePressures: TirePressures{UpdatedAt: time.Now()},
			},
			want: float64Ptr(100.0),
		},
		{
			name: "offline - no score",
			state: &VehicleState{
				IsOnline: false,
			},
			want: nil,
		},
		{
			name: "low battery",
			state: &VehicleState{
				IsOnline:      true,
				BatteryLevel:  20,
				RangeEstimate: 50,
				Doors:         Closures{ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				Windows:       Closures{ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				Frunk:         ClosureStatusClosed,
				Liftgate:      ClosureStatusClosed,
				IsLocked:      true,
				TirePressures: TirePressures{UpdatedAt: time.Now()},
			},
			want: func() *float64 {
				// Battery: 20 * 0.4 = 8
				// Range: (50/300*100) * 0.2 = 3.33
				// Closures: 100 * 0.2 = 20
				// Lock: 100 * 0.1 = 10
				// Tires: 100 * 0.1 = 10
				// Total: ~51.3
				v := 51.3
				return &v
			}(),
		},
		{
			name: "door open penalty",
			state: &VehicleState{
				IsOnline:      true,
				BatteryLevel:  100,
				RangeEstimate: 300,
				Doors:         Closures{ClosureStatusOpen, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				Windows:       Closures{ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				Frunk:         ClosureStatusClosed,
				Liftgate:      ClosureStatusClosed,
				IsLocked:      false,
				TirePressures: TirePressures{UpdatedAt: time.Now()},
			},
			want: func() *float64 {
				// Battery: 100 * 0.4 = 40
				// Range: 100 * 0.2 = 20
				// Closures: (100 - 50) * 0.2 = 10 (door open penalty)
				// Lock: 0 * 0.1 = 0 (unlocked)
				// Tires: 100 * 0.1 = 10
				// Total: 80
				v := 80.0
				return &v
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.CalculateReadyScore()

			if tt.want == nil {
				if got != nil {
					t.Errorf("CalculateReadyScore() = %v, want nil", *got)
				}
				return
			}

			if got == nil {
				t.Fatalf("CalculateReadyScore() = nil, want %v", *tt.want)
			}

			// Allow small floating point differences
			diff := *got - *tt.want
			if diff < -1.0 || diff > 1.0 {
				t.Errorf("CalculateReadyScore() = %v, want %v (diff: %v)", *got, *tt.want, diff)
			}
		})
	}
}

func TestNeedsCharge(t *testing.T) {
	tests := []struct {
		name  string
		state *VehicleState
		want  bool
	}{
		{
			name: "below limit",
			state: &VehicleState{
				BatteryLevel: 70,
				ChargeLimit:  80,
			},
			want: true,
		},
		{
			name: "at limit",
			state: &VehicleState{
				BatteryLevel: 80,
				ChargeLimit:  80,
			},
			want: false,
		},
		{
			name: "above limit",
			state: &VehicleState{
				BatteryLevel: 90,
				ChargeLimit:  80,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.NeedsCharge(); got != tt.want {
				t.Errorf("NeedsCharge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCharging(t *testing.T) {
	tests := []struct {
		name  string
		state ChargeState
		want  bool
	}{
		{"charging", ChargeStateCharging, true},
		{"not charging", ChargeStateNotCharging, false},
		{"complete", ChargeStateComplete, false},
		{"disconnected", ChargeStateDisconnected, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &VehicleState{ChargeState: tt.state}
			if got := state.IsCharging(); got != tt.want {
				t.Errorf("IsCharging() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasCriticalIssues(t *testing.T) {
	tests := []struct {
		name  string
		state *VehicleState
		want  bool
	}{
		{
			name: "critical range",
			state: &VehicleState{
				RangeStatus: RangeStatusCritical,
			},
			want: true,
		},
		{
			name: "low range (not critical)",
			state: &VehicleState{
				RangeStatus: RangeStatusLow,
			},
			want: false,
		},
		{
			name: "locked with door open (impossible state)",
			state: &VehicleState{
				IsLocked:    true,
				Doors:       Closures{ClosureStatusOpen, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				RangeStatus: RangeStatusNormal,
			},
			want: true,
		},
		{
			name: "normal state",
			state: &VehicleState{
				IsLocked:    true,
				Doors:       Closures{ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				RangeStatus: RangeStatusNormal,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.HasCriticalIssues(); got != tt.want {
				t.Errorf("HasCriticalIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIssues(t *testing.T) {
	tests := []struct {
		name       string
		state      *VehicleState
		wantCount  int
		wantAny    string // Check if any issue contains this string
	}{
		{
			name: "critical range",
			state: &VehicleState{
				IsOnline:      true,
				RangeStatus:   RangeStatusCritical,
				RangeEstimate: 20,
			},
			wantCount: 1,
			wantAny:   "Critical",
		},
		{
			name: "low range",
			state: &VehicleState{
				IsOnline:      true,
				RangeStatus:   RangeStatusLow,
				RangeEstimate: 40,
			},
			wantCount: 1,
			wantAny:   "Low range",
		},
		{
			name: "needs charge",
			state: &VehicleState{
				IsOnline:     true,
				BatteryLevel: 50,
				ChargeLimit:  80,
				ChargeState:  ChargeStateNotCharging,
				RangeStatus:  RangeStatusNormal,
			},
			wantCount: 1,
			wantAny:   "below charge limit",
		},
		{
			name: "door open",
			state: &VehicleState{
				IsOnline:    true,
				Doors:       Closures{ClosureStatusOpen, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				RangeStatus: RangeStatusNormal,
			},
			wantCount: 1,
			wantAny:   "doors open",
		},
		{
			name: "offline",
			state: &VehicleState{
				IsOnline:    false,
				RangeStatus: RangeStatusNormal,
			},
			wantCount: 1,
			wantAny:   "offline",
		},
		{
			name: "no issues",
			state: &VehicleState{
				IsOnline:      true,
				RangeStatus:   RangeStatusNormal,
				BatteryLevel:  90,
				ChargeLimit:   80,
				Doors:         Closures{ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				Windows:       Closures{ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed, ClosureStatusClosed},
				Frunk:         ClosureStatusClosed,
				Liftgate:      ClosureStatusClosed,
				IsLocked:      true,
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := tt.state.GetIssues()
			if len(issues) != tt.wantCount {
				t.Errorf("GetIssues() returned %d issues, want %d. Issues: %v", len(issues), tt.wantCount, issues)
			}

			if tt.wantAny != "" {
				found := false
				for _, issue := range issues {
					if containsIgnoreCase(issue, tt.wantAny) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetIssues() should contain '%s', got %v", tt.wantAny, issues)
				}
			}
		})
	}
}

func TestEstimatedChargeTime(t *testing.T) {
	now := time.Now()
	future := now.Add(2 * time.Hour)

	tests := []struct {
		name  string
		state *VehicleState
		want  *float64
	}{
		{
			name: "charging with time estimate",
			state: &VehicleState{
				ChargeState:  ChargeStateCharging,
				TimeToCharge: &future,
				UpdatedAt:    now,
			},
			want: float64Ptr(2.0),
		},
		{
			name: "not charging",
			state: &VehicleState{
				ChargeState: ChargeStateNotCharging,
			},
			want: nil,
		},
		{
			name: "charging but no time estimate",
			state: &VehicleState{
				ChargeState: ChargeStateCharging,
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.EstimatedChargeTime()

			if tt.want == nil {
				if got != nil {
					t.Errorf("EstimatedChargeTime() = %v, want nil", *got)
				}
				return
			}

			if got == nil {
				t.Fatalf("EstimatedChargeTime() = nil, want %v", *tt.want)
			}

			// Allow small differences
			diff := *got - *tt.want
			if diff < -0.1 || diff > 0.1 {
				t.Errorf("EstimatedChargeTime() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

// Helper functions

func float64Ptr(v float64) *float64 {
	return &v
}

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
