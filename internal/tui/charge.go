package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pfrederiksen/rivian-ls/internal/model"
)

// ChargeView handles the charging details display
type ChargeView struct{}

// NewChargeView creates a new charge view
func NewChargeView() *ChargeView {
	return &ChargeView{}
}

// Render renders the charge view
func (v *ChargeView) Render(state *model.VehicleState, width, height int) string {
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

	// Main charging status section
	statusSection := v.renderChargingStatus(state, sectionStyle, labelStyle, valueStyle)

	// Battery details section
	batterySection := v.renderBatteryDetails(state, sectionStyle, labelStyle, valueStyle)

	// Charging recommendations
	recommendationsSection := v.renderRecommendations(state, sectionStyle, labelStyle, valueStyle)

	// Arrange in columns
	leftColumn := lipgloss.JoinVertical(
		lipgloss.Left,
		statusSection,
		recommendationsSection,
	)

	rightColumn := batterySection

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumn,
		"  ",
		rightColumn,
	)

	return titleStyle.Render("🔋 Charging") + "\n" + content
}

func (v *ChargeView) renderChargingStatus(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	// Charge state with large emoji
	stateEmoji := "🔌"
	stateText := "Not Plugged In"
	stateColor := lipgloss.Color("#888888")

	switch state.ChargeState {
	case model.ChargeStateCharging:
		stateEmoji = "⚡"
		stateText = "Charging"
		stateColor = lipgloss.Color("#00ff00")
	case model.ChargeStateComplete:
		stateEmoji = "✓"
		stateText = "Charge Complete"
		stateColor = lipgloss.Color("#00ff00")
	case model.ChargeStateScheduled:
		stateEmoji = "⏱"
		stateText = "Scheduled"
		stateColor = lipgloss.Color("#ffff00")
	case model.ChargeStateDisconnected:
		stateEmoji = "🔌"
		stateText = "Disconnected"
		stateColor = lipgloss.Color("#888888")
	case model.ChargeStateNotCharging:
		stateEmoji = "○"
		stateText = "Not Charging"
		stateColor = lipgloss.Color("#888888")
	}

	emojiStyle := lipgloss.NewStyle().
		Foreground(stateColor).
		Bold(true).
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Foreground(stateColor).
		Bold(true).
		Align(lipgloss.Center).
		MarginTop(1).
		MarginBottom(2)

	content := emojiStyle.Render(fmt.Sprintf("    %s", stateEmoji)) + "\n"
	content += titleStyle.Render(stateText) + "\n\n"

	// Current battery level with large bar
	batteryBar := v.renderBatteryBar(state.BatteryLevel, 30)
	content += fmt.Sprintf("%s\n\n",
		lipgloss.NewStyle().Align(lipgloss.Center).Render(batteryBar),
	)

	percentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true).
		Align(lipgloss.Center)

	content += percentStyle.Render(fmt.Sprintf("%.1f%%", state.BatteryLevel)) + "\n\n"

	// Charging details (if charging)
	if state.ChargeState == model.ChargeStateCharging {
		if state.ChargingRate != nil && *state.ChargingRate > 0 {
			content += fmt.Sprintf("%s %s\n",
				labelStyle.Render("Charging Rate:"),
				valueStyle.Render(fmt.Sprintf("%.1f kW", *state.ChargingRate)),
			)
		}

		if state.TimeToCharge != nil {
			remaining := state.TimeToCharge.Sub(state.UpdatedAt)
			if remaining > 0 {
				hours := int(remaining.Hours())
				minutes := int(remaining.Minutes()) % 60
				content += fmt.Sprintf("%s %s\n",
					labelStyle.Render("Time Remaining:"),
					valueStyle.Render(fmt.Sprintf("%dh %dm", hours, minutes)),
				)

				estComplete := state.TimeToCharge.Format("3:04 PM")
				content += fmt.Sprintf("%s %s\n",
					labelStyle.Render("Est. Complete:"),
					valueStyle.Render(estComplete),
				)
			}
		}

		// Calculate energy being added
		if state.ChargingRate != nil && state.BatteryCapacity > 0 {
			neededKWh := (float64(state.ChargeLimit) - state.BatteryLevel) / 100.0 * state.BatteryCapacity
			content += fmt.Sprintf("\n%s %s",
				labelStyle.Render("Energy to Add:"),
				valueStyle.Render(fmt.Sprintf("%.1f kWh", neededKWh)),
			)
		}
	}

	return sectionStyle.Width(40).Render(content)
}

func (v *ChargeView) renderBatteryDetails(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	content := fmt.Sprintf("%s %s\n\n",
		labelStyle.Render("Current Level:"),
		valueStyle.Render(fmt.Sprintf("%.1f%%", state.BatteryLevel)),
	)

	content += fmt.Sprintf("%s %s\n\n",
		labelStyle.Render("Charge Limit:"),
		valueStyle.Render(fmt.Sprintf("%d%%", state.ChargeLimit)),
	)

	// Calculate how much more charge is needed
	neededPercent := float64(state.ChargeLimit) - state.BatteryLevel
	if neededPercent > 0 {
		content += fmt.Sprintf("%s %s\n\n",
			labelStyle.Render("To Limit:"),
			valueStyle.Render(fmt.Sprintf("+%.1f%%", neededPercent)),
		)
	} else {
		content += fmt.Sprintf("%s %s\n\n",
			labelStyle.Render("Status:"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render("At limit"),
		)
	}

	// Battery capacity (if available)
	if state.BatteryCapacity > 0 {
		content += fmt.Sprintf("%s %s\n\n",
			labelStyle.Render("Capacity:"),
			valueStyle.Render(fmt.Sprintf("%.1f kWh", state.BatteryCapacity)),
		)

		// Calculate current energy
		currentEnergy := state.BatteryLevel / 100.0 * state.BatteryCapacity
		content += fmt.Sprintf("%s %s\n",
			labelStyle.Render("Current Energy:"),
			valueStyle.Render(fmt.Sprintf("%.1f kWh", currentEnergy)),
		)
	}

	// Range
	rangeColor := lipgloss.Color("#00ff00")
	switch state.RangeStatus {
	case model.RangeStatusLow:
		rangeColor = lipgloss.Color("#ffff00")
	case model.RangeStatusCritical:
		rangeColor = lipgloss.Color("#ff0000")
	}
	rangeStyle := valueStyle.Foreground(rangeColor)

	content += fmt.Sprintf("\n%s %s",
		labelStyle.Render("Range:"),
		rangeStyle.Render(fmt.Sprintf("%.0f mi (%s)", state.RangeEstimate, state.RangeStatus)),
	)

	return sectionStyle.Width(30).Render("📊 Battery Details\n\n" + content)
}

func (v *ChargeView) renderRecommendations(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	recommendations := v.getChargingRecommendations(state)

	if len(recommendations) == 0 {
		return ""
	}

	content := ""
	for i, rec := range recommendations {
		if i > 0 {
			content += "\n\n"
		}

		icon := "💡"
		if rec.critical {
			icon = "⚠️"
		}

		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffff00"))
		if rec.critical {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
		}

		content += fmt.Sprintf("%s %s", icon, style.Render(rec.message))
	}

	return sectionStyle.Width(40).Render("💡 Recommendations\n\n" + content)
}

type recommendation struct {
	message  string
	critical bool
}

func (v *ChargeView) getChargingRecommendations(state *model.VehicleState) []recommendation {
	var recs []recommendation

	// Critical battery warning
	if state.RangeStatus == model.RangeStatusCritical {
		recs = append(recs, recommendation{
			message:  fmt.Sprintf("Critical battery level! Only %.0f miles remaining", state.RangeEstimate),
			critical: true,
		})
	}

	// Low battery warning
	if state.RangeStatus == model.RangeStatusLow && state.ChargeState != model.ChargeStateCharging {
		recs = append(recs, recommendation{
			message:  "Low battery - consider charging soon",
			critical: false,
		})
	}

	// Below charge limit
	if state.BatteryLevel < float64(state.ChargeLimit) && state.ChargeState != model.ChargeStateCharging {
		recs = append(recs, recommendation{
			message:  fmt.Sprintf("Battery below limit (%d%%) - connect to charger", state.ChargeLimit),
			critical: false,
		})
	}

	// Optimal charging range (20-80%)
	if state.BatteryLevel > 85 {
		recs = append(recs, recommendation{
			message:  "Battery above 85% - consider setting lower charge limit for battery health",
			critical: false,
		})
	}

	// Charge complete but still plugged in
	if state.ChargeState == model.ChargeStateComplete && state.BatteryLevel >= float64(state.ChargeLimit) {
		recs = append(recs, recommendation{
			message:  "Charge complete - vehicle ready to drive",
			critical: false,
		})
	}

	return recs
}

// renderBatteryBar creates a visual battery bar
func (v *ChargeView) renderBatteryBar(level float64, width int) string {
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
