package model

import "math"

// CalculateReadyScore computes a 0-100 "readiness to drive" score.
//
// Factors:
// - Battery level (40% weight): Higher is better
// - Range (20% weight): More miles = higher score
// - Closures (20% weight): All closed = 100%, any open = penalty
// - Lock status (10% weight): Locked = 100%
// - Tire pressure (10% weight): All normal = 100%
//
// Returns nil if insufficient data to calculate.
func (v *VehicleState) CalculateReadyScore() *float64 {
	if !v.IsOnline {
		return nil // Can't calculate if vehicle is offline
	}

	var score float64 = 0

	// Battery level (40% weight)
	batteryScore := v.BatteryLevel // Already 0-100
	score += batteryScore * 0.4

	// Range (20% weight)
	// Consider 300+ miles as "perfect" range
	rangeScore := math.Min(v.RangeEstimate/300.0*100, 100)
	score += rangeScore * 0.2

	// Closures (20% weight)
	closureScore := 100.0
	if v.Doors.AnyOpen() {
		closureScore -= 50 // Significant penalty for open doors
	}
	if v.Windows.AnyOpen() {
		closureScore -= 25 // Moderate penalty for open windows
	}
	if v.Frunk == ClosureStatusOpen {
		closureScore -= 12.5
	}
	if v.Liftgate == ClosureStatusOpen {
		closureScore -= 12.5
	}
	closureScore = math.Max(closureScore, 0)
	score += closureScore * 0.2

	// Lock status (10% weight)
	lockScore := 0.0
	if v.IsLocked {
		lockScore = 100.0
	}
	score += lockScore * 0.1

	// Tire pressure (10% weight)
	// Note: Rivian API returns status, not PSI
	// For now, assume 100% if we have any data
	tireScore := 100.0
	if v.TirePressures.UpdatedAt.IsZero() {
		tireScore = 50.0 // Penalty for no tire data
	}
	score += tireScore * 0.1

	// Round to 1 decimal place
	score = math.Round(score*10) / 10

	return &score
}

// UpdateReadyScore recalculates and updates the ReadyScore field.
func (v *VehicleState) UpdateReadyScore() {
	v.ReadyScore = v.CalculateReadyScore()
}

// NeedsCharge returns true if battery is below the charge limit.
func (v *VehicleState) NeedsCharge() bool {
	return v.BatteryLevel < float64(v.ChargeLimit)
}

// IsCharging returns true if currently charging.
func (v *VehicleState) IsCharging() bool {
	return v.ChargeState == ChargeStateCharging
}

// HasCriticalIssues returns true if any critical issues are detected.
func (v *VehicleState) HasCriticalIssues() bool {
	// Critical: Low range
	if v.RangeStatus == RangeStatusCritical {
		return true
	}

	// Critical: Doors open while locked should be impossible
	if v.IsLocked && v.Doors.AnyOpen() {
		return true
	}

	return false
}

// GetIssues returns a list of current issues/warnings.
func (v *VehicleState) GetIssues() []string {
	var issues []string

	// Range warnings
	switch v.RangeStatus {
	case RangeStatusCritical:
		issues = append(issues, "Critical: Range below 25 miles")
	case RangeStatusLow:
		issues = append(issues, "Warning: Low range (< 50 miles)")
	}

	// Battery below charge limit
	if v.NeedsCharge() && !v.IsCharging() {
		issues = append(issues, "Battery below charge limit - connect to charger")
	}

	// Closure warnings
	if v.Doors.AnyOpen() {
		issues = append(issues, "Warning: One or more doors open")
	}
	if v.Windows.AnyOpen() {
		issues = append(issues, "Warning: One or more windows open")
	}
	if v.Frunk == ClosureStatusOpen {
		issues = append(issues, "Warning: Frunk open")
	}
	if v.Liftgate == ClosureStatusOpen {
		issues = append(issues, "Warning: Liftgate open")
	}
	if v.TonneauCover != nil && *v.TonneauCover == ClosureStatusOpen {
		issues = append(issues, "Warning: Tonneau cover open")
	}

	// Lock status
	if !v.IsLocked && v.Doors.AllClosed() && v.Windows.AllClosed() {
		issues = append(issues, "Info: Vehicle unlocked")
	}

	// Offline warning
	if !v.IsOnline {
		issues = append(issues, "Warning: Vehicle offline")
	}

	return issues
}

// EstimatedChargeTime returns estimated time remaining to charge to limit.
// Returns nil if not charging or no time estimate available.
func (v *VehicleState) EstimatedChargeTime() *float64 {
	if v.TimeToCharge == nil || !v.IsCharging() {
		return nil
	}

	// Calculate hours remaining
	hours := v.TimeToCharge.Sub(v.UpdatedAt).Hours()
	if hours < 0 {
		hours = 0
	}

	return &hours
}
