package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pfrederiksen/rivian-ls/internal/model"
)

// DashboardView handles the main dashboard display
type DashboardView struct{}

// NewDashboardView creates a new dashboard view
func NewDashboardView() *DashboardView {
	return &DashboardView{}
}

// Render renders the dashboard view
func (v *DashboardView) Render(state *model.VehicleState, width, height int) string {
	// Define styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ffff")).
		Bold(true).
		MarginTop(1).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(1).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true)

	// Battery & Range Section
	batterySection := v.renderBatterySection(state, sectionStyle, labelStyle, valueStyle)

	// Charging Section
	chargingSection := v.renderChargingSection(state, sectionStyle, labelStyle, valueStyle)

	// Security Section
	securitySection := v.renderSecuritySection(state, sectionStyle, labelStyle, valueStyle)

	// Location & Stats Section
	statsSection := v.renderStatsSection(state, sectionStyle, labelStyle, valueStyle)

	// Tire Status Section
	tiresSection := v.renderTirePressures(state, sectionStyle, labelStyle, valueStyle)

	// Battery Stats Section (calculated from current data)
	vehicleSection := v.renderVehicleInfo(state, sectionStyle, labelStyle, valueStyle)

	// Ready Score Section (if available)
	var readySection string
	if state.ReadyScore != nil {
		readySection = v.renderReadyScore(state, sectionStyle, labelStyle, valueStyle)
	}

	// Issues Section (if any)
	issuesSection := v.renderIssues(state, sectionStyle)

	// Arrange sections in a three-column grid layout
	leftColumn := lipgloss.JoinVertical(
		lipgloss.Left,
		batterySection,
		chargingSection,
	)

	middleColumn := lipgloss.JoinVertical(
		lipgloss.Left,
		securitySection,
		tiresSection,
	)

	rightColumn := lipgloss.JoinVertical(
		lipgloss.Left,
		statsSection,
		vehicleSection,
	)

	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumn,
		"  ",
		middleColumn,
		"  ",
		rightColumn,
	)

	bottomRow := ""
	if readySection != "" {
		bottomRow = readySection
	}
	if issuesSection != "" {
		if bottomRow != "" {
			bottomRow += "\n"
		}
		bottomRow += issuesSection
	}

	return titleStyle.Render("📊 Dashboard") + "\n" +
		topRow +
		"\n" + bottomRow
}

func (v *DashboardView) renderBatterySection(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	// Battery level with bar
	batteryBar := v.renderBatteryBar(state.BatteryLevel, 20)

	// Range with color based on status
	rangeColor := lipgloss.Color("#00ff00")
	switch state.RangeStatus {
	case model.RangeStatusLow:
		rangeColor = lipgloss.Color("#ffff00")
	case model.RangeStatusCritical:
		rangeColor = lipgloss.Color("#ff0000")
	}
	rangeStyle := valueStyle.Foreground(rangeColor)

	content := fmt.Sprintf("%s %s\n\n",
		labelStyle.Render("Battery:"),
		valueStyle.Render(fmt.Sprintf("%.1f%%", state.BatteryLevel)),
	)
	content += batteryBar + "\n\n"
	content += fmt.Sprintf("%s %s (%s)\n",
		labelStyle.Render("Range:"),
		rangeStyle.Render(fmt.Sprintf("%.0f mi", state.RangeEstimate)),
		state.RangeStatus,
	)
	content += fmt.Sprintf("%s %d%%",
		labelStyle.Render("Charge Limit:"),
		state.ChargeLimit,
	)

	return sectionStyle.Width(35).Render("⚡ Battery & Range\n\n" + content)
}

func (v *DashboardView) renderChargingSection(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	// Charge state with emoji
	stateEmoji := "🔌"
	stateText := string(state.ChargeState)
	stateColor := lipgloss.Color("#ffffff")

	switch state.ChargeState {
	case model.ChargeStateCharging:
		stateEmoji = "⚡"
		stateText = "Charging"
		stateColor = lipgloss.Color("#00ff00")
	case model.ChargeStateComplete:
		stateEmoji = "✓"
		stateText = "Complete"
		stateColor = lipgloss.Color("#00ff00")
	case model.ChargeStateDisconnected:
		stateEmoji = "🔌"
		stateText = "Disconnected"
		stateColor = lipgloss.Color("#888888")
	case model.ChargeStateScheduled:
		stateEmoji = "⏱"
		stateText = "Scheduled"
		stateColor = lipgloss.Color("#ffff00")
	case model.ChargeStateNotCharging:
		stateEmoji = "○"
		stateText = "Not Charging"
		stateColor = lipgloss.Color("#888888")
	case model.ChargeStateUnknown, "":
		stateEmoji = "❓"
		stateText = "Unknown"
		stateColor = lipgloss.Color("#888888")
	}
	stateStyle := valueStyle.Foreground(stateColor)

	content := fmt.Sprintf("%s %s %s\n\n",
		labelStyle.Render("Status:"),
		stateEmoji,
		stateStyle.Render(stateText),
	)

	// Charging details only if actively charging
	if state.ChargeState == model.ChargeStateCharging {
		if state.ChargingRate != nil && *state.ChargingRate > 0 {
			content += fmt.Sprintf("%s %s\n",
				labelStyle.Render("Rate:"),
				valueStyle.Render(fmt.Sprintf("%.1f kW", *state.ChargingRate)),
			)
		}

		if state.TimeToCharge != nil {
			remaining := state.TimeToCharge.Sub(state.UpdatedAt)
			if remaining > 0 {
				hours := int(remaining.Hours())
				minutes := int(remaining.Minutes()) % 60
				content += fmt.Sprintf("%s %s",
					labelStyle.Render("Time:"),
					valueStyle.Render(fmt.Sprintf("%dh %dm", hours, minutes)),
				)
			}
		}
	}

	return sectionStyle.Width(35).Render("🔋 Charging\n\n" + content)
}

func (v *DashboardView) renderSecuritySection(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	// Lock status
	lockEmoji := "🔒"
	lockStatus := "Locked"
	lockColor := lipgloss.Color("#00ff00")
	if !state.IsLocked {
		lockEmoji = "🔓"
		lockStatus = "Unlocked"
		lockColor = lipgloss.Color("#ffff00")
	}
	lockStyle := valueStyle.Foreground(lockColor)

	content := fmt.Sprintf("%s %s %s\n\n",
		labelStyle.Render("Lock:"),
		lockEmoji,
		lockStyle.Render(lockStatus),
	)

	// Doors
	doorsStatus := "All closed"
	if state.Doors.AnyOpen() {
		count := 0
		if state.Doors.FrontLeft == model.ClosureStatusOpen {
			count++
		}
		if state.Doors.FrontRight == model.ClosureStatusOpen {
			count++
		}
		if state.Doors.RearLeft == model.ClosureStatusOpen {
			count++
		}
		if state.Doors.RearRight == model.ClosureStatusOpen {
			count++
		}
		doorsStatus = fmt.Sprintf("%d open", count)
	}
	content += fmt.Sprintf("%s %s\n",
		labelStyle.Render("Doors:"),
		valueStyle.Render(doorsStatus),
	)

	// Windows
	windowsStatus := "All closed"
	if state.Windows.AnyOpen() {
		count := 0
		if state.Windows.FrontLeft == model.ClosureStatusOpen {
			count++
		}
		if state.Windows.FrontRight == model.ClosureStatusOpen {
			count++
		}
		if state.Windows.RearLeft == model.ClosureStatusOpen {
			count++
		}
		if state.Windows.RearRight == model.ClosureStatusOpen {
			count++
		}
		windowsStatus = fmt.Sprintf("%d open", count)
	}
	content += fmt.Sprintf("%s %s",
		labelStyle.Render("Windows:"),
		valueStyle.Render(windowsStatus),
	)

	return sectionStyle.Width(35).Render("🔐 Security\n\n" + content)
}

func (v *DashboardView) renderStatsSection(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	content := ""

	// Temperature
	if state.CabinTemp != nil {
		tempColor := lipgloss.Color("#00ff00")
		temp := *state.CabinTemp
		if temp < 60 || temp > 80 {
			tempColor = lipgloss.Color("#ffff00")
		}
		if temp < 40 || temp > 90 {
			tempColor = lipgloss.Color("#ff0000")
		}
		tempStyle := valueStyle.Foreground(tempColor)
		content += fmt.Sprintf("%s %s\n",
			labelStyle.Render("Cabin:"),
			tempStyle.Render(fmt.Sprintf("%.1f°F", temp)),
		)
	}
	if state.ExteriorTemp != nil {
		content += fmt.Sprintf("%s %s\n\n",
			labelStyle.Render("Exterior:"),
			valueStyle.Render(fmt.Sprintf("%.1f°F", *state.ExteriorTemp)),
		)
	}

	// Odometer
	content += fmt.Sprintf("%s %s\n\n",
		labelStyle.Render("Odometer:"),
		valueStyle.Render(fmt.Sprintf("%.1f mi", state.Odometer)),
	)

	// Location (if available)
	if state.Location != nil {
		content += fmt.Sprintf("%s\n%s\n",
			labelStyle.Render("Location:"),
			valueStyle.Render(fmt.Sprintf("%.4f°N, %.4f°W", state.Location.Latitude, state.Location.Longitude)),
		)
	}

	return sectionStyle.Width(30).Render("🌡️  Climate & Travel\n\n" + content)
}

func (v *DashboardView) renderTirePressures(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	// Helper to render tire status with color
	renderTireStatus := func(label string, status model.TirePressureStatus) string {
		statusColor := lipgloss.Color("#00ff00") // Green for OK
		if status == model.TirePressureStatusLow {
			statusColor = lipgloss.Color("#ffff00") // Yellow for low
		} else if status == model.TirePressureStatusHigh {
			statusColor = lipgloss.Color("#ff8800") // Orange for high
		} else if status == model.TirePressureStatusUnknown || status == "" {
			statusColor = lipgloss.Color("#888888") // Gray for unknown
		}
		statusStyle := valueStyle.Foreground(statusColor)

		displayStatus := string(status)
		if displayStatus == "" {
			displayStatus = "unknown"
		}

		return fmt.Sprintf("%s %s",
			labelStyle.Render(label+":"),
			statusStyle.Render(displayStatus),
		)
	}

	// Front tires
	content := renderTireStatus("Front Left", state.TirePressures.FrontLeftStatus) + "\n"
	content += renderTireStatus("Front Right", state.TirePressures.FrontRightStatus) + "\n\n"

	// Rear tires
	content += renderTireStatus("Rear Left", state.TirePressures.RearLeftStatus) + "\n"
	content += renderTireStatus("Rear Right", state.TirePressures.RearRightStatus)

	return sectionStyle.Width(35).Render("🚗 Tire Status\n\n" + content)
}

func (v *DashboardView) renderVehicleInfo(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	content := ""

	// Battery capacity
	if state.BatteryCapacity > 0 {
		content += fmt.Sprintf("%s %s\n\n",
			labelStyle.Render("Capacity:"),
			valueStyle.Render(fmt.Sprintf("%.1f kWh", state.BatteryCapacity)),
		)

		// Calculate current energy
		currentEnergy := state.BatteryLevel / 100.0 * state.BatteryCapacity
		content += fmt.Sprintf("%s %s\n\n",
			labelStyle.Render("Current Energy:"),
			valueStyle.Render(fmt.Sprintf("%.1f kWh", currentEnergy)),
		)

		// Calculate efficiency (mi/kWh)
		if state.RangeEstimate > 0 {
			efficiency := state.RangeEstimate / currentEnergy
			content += fmt.Sprintf("%s %s\n\n",
				labelStyle.Render("Efficiency:"),
				valueStyle.Render(fmt.Sprintf("%.2f mi/kWh", efficiency)),
			)
		}

		// Calculate mi/%
		milesPerPercent := state.RangeEstimate / state.BatteryLevel
		content += fmt.Sprintf("%s %s",
			labelStyle.Render("mi/%:"),
			valueStyle.Render(fmt.Sprintf("%.2f mi/%%", milesPerPercent)),
		)
	}

	return sectionStyle.Width(30).Render("⚡ Battery Stats\n\n" + content)
}

func (v *DashboardView) renderReadyScore(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	if state.ReadyScore == nil {
		return ""
	}

	score := *state.ReadyScore
	scoreBar := v.renderScoreBar(score, 40)

	// Color based on score
	scoreColor := lipgloss.Color("#00ff00")
	if score < 50 {
		scoreColor = lipgloss.Color("#ff0000")
	} else if score < 75 {
		scoreColor = lipgloss.Color("#ffff00")
	}
	scoreStyle := valueStyle.Foreground(scoreColor)

	content := fmt.Sprintf("%s %s / 100\n\n",
		labelStyle.Render("Score:"),
		scoreStyle.Render(fmt.Sprintf("%.1f", score)),
	)
	content += scoreBar

	return sectionStyle.Width(72).Render("🎯 Ready Score\n\n" + content)
}

func (v *DashboardView) renderIssues(state *model.VehicleState, sectionStyle lipgloss.Style) string {
	issues := state.GetIssues()
	if len(issues) == 0 {
		return ""
	}

	issueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffff00"))

	criticalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000")).
		Bold(true)

	var content strings.Builder
	for _, issue := range issues {
		// Use critical style for critical issues
		if strings.Contains(strings.ToLower(issue), "critical") {
			content.WriteString(criticalStyle.Render("⚠ " + issue))
		} else if strings.HasPrefix(issue, "Info:") {
			// Info items in gray
			content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("ℹ " + strings.TrimPrefix(issue, "Info: ")))
		} else {
			content.WriteString(issueStyle.Render("• " + issue))
		}
		content.WriteString("\n")
	}

	return sectionStyle.Width(72).Render("⚠️  Issues\n\n" + content.String())
}

// renderBatteryBar creates a visual battery bar
func (v *DashboardView) renderBatteryBar(level float64, width int) string {
	filled := int(level * float64(width) / 100)
	if filled > width {
		filled = width
	}
	empty := width - filled

	// Color based on level
	barColor := lipgloss.Color("#00ff00")
	if level < 20 {
		barColor = lipgloss.Color("#ff0000")
	} else if level < 50 {
		barColor = lipgloss.Color("#ffff00")
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))

	return fmt.Sprintf("[%s]", bar)
}

// renderScoreBar creates a visual score bar
func (v *DashboardView) renderScoreBar(score float64, width int) string {
	filled := int(score * float64(width) / 100)
	if filled > width {
		filled = width
	}
	empty := width - filled

	// Color based on score
	barColor := lipgloss.Color("#00ff00")
	if score < 50 {
		barColor = lipgloss.Color("#ff0000")
	} else if score < 75 {
		barColor = lipgloss.Color("#ffff00")
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))

	return fmt.Sprintf("[%s]", bar)
}
