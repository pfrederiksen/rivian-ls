package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/store"
)

// HealthView handles the health and history display
type HealthView struct {
	store     *store.Store
	vehicleID string
	history   []*model.VehicleState // Cache of recent history
}

// NewHealthView creates a new health view
func NewHealthView(store *store.Store, vehicleID string) *HealthView {
	return &HealthView{
		store:     store,
		vehicleID: vehicleID,
	}
}

// Render renders the health view
func (v *HealthView) Render(state *model.VehicleState, width, height int) string {
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

	// Load recent history if not already loaded
	if v.history == nil && v.store != nil {
		ctx := context.Background()
		// Get history from last 7 days, limit to 20 states
		history, err := v.store.GetStateHistory(ctx, v.vehicleID, time.Now().Add(-7*24*time.Hour), 20)
		if err == nil {
			v.history = history
		}
	}

	// Current health status
	healthSection := v.renderHealthStatus(state, sectionStyle, labelStyle, valueStyle)

	// Recent history trends
	trendsSection := v.renderTrends(state, sectionStyle, labelStyle, valueStyle)

	// Diagnostics
	diagnosticsSection := v.renderDiagnostics(state, sectionStyle, labelStyle, valueStyle)

	// Arrange sections
	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		healthSection,
		"  ",
		trendsSection,
	)

	return titleStyle.Render("🏥 Vehicle Health") + "\n" +
		topRow + "\n" +
		diagnosticsSection
}

func (v *HealthView) renderHealthStatus(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	// Overall health indicator
	healthEmoji := "✓"
	healthText := "Healthy"
	healthColor := lipgloss.Color("#00ff00")

	if state.HasCriticalIssues() {
		healthEmoji = "⚠️"
		healthText = "Needs Attention"
		healthColor = lipgloss.Color("#ff0000")
	} else if len(state.GetIssues()) > 0 {
		healthEmoji = "⚠"
		healthText = "Minor Issues"
		healthColor = lipgloss.Color("#ffff00")
	}

	statusStyle := lipgloss.NewStyle().
		Foreground(healthColor).
		Bold(true).
		Align(lipgloss.Center)

	content := statusStyle.Render(fmt.Sprintf("    %s", healthEmoji)) + "\n"
	content += statusStyle.Render(healthText) + "\n\n"

	// Online status
	onlineIcon := "🟢"
	onlineText := "Online"
	if !state.IsOnline {
		onlineIcon = "🔴"
		onlineText = "Offline"
	}
	content += fmt.Sprintf("%s %s\n\n",
		labelStyle.Render("Status:"),
		valueStyle.Render(fmt.Sprintf("%s %s", onlineIcon, onlineText)),
	)

	// Ready Score (if available)
	if state.ReadyScore != nil {
		scoreColor := lipgloss.Color("#00ff00")
		if *state.ReadyScore < 50 {
			scoreColor = lipgloss.Color("#ff0000")
		} else if *state.ReadyScore < 75 {
			scoreColor = lipgloss.Color("#ffff00")
		}
		scoreStyle := valueStyle.Foreground(scoreColor)

		content += fmt.Sprintf("%s %s\n\n",
			labelStyle.Render("Ready Score:"),
			scoreStyle.Render(fmt.Sprintf("%.1f / 100", *state.ReadyScore)),
		)
	}

	// Odometer
	content += fmt.Sprintf("%s %s\n",
		labelStyle.Render("Odometer:"),
		valueStyle.Render(fmt.Sprintf("%.1f mi", state.Odometer)),
	)

	// Last update
	timeAgo := time.Since(state.UpdatedAt)
	timeText := formatDuration(timeAgo)
	content += fmt.Sprintf("%s %s",
		labelStyle.Render("Last Update:"),
		valueStyle.Render(timeText),
	)

	return sectionStyle.Width(35).Render("🩺 Current Status\n\n" + content)
}

func (v *HealthView) renderTrends(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	if len(v.history) < 2 {
		return sectionStyle.Width(35).Render("📈 Trends\n\nInsufficient data for trend analysis")
	}

	content := ""

	// Battery trend
	batteryTrend := v.calculateTrend("battery")
	content += fmt.Sprintf("%s\n%s\n\n",
		labelStyle.Render("Battery:"),
		v.renderTrendIndicator(batteryTrend),
	)

	// Range trend
	rangeTrend := v.calculateTrend("range")
	content += fmt.Sprintf("%s\n%s\n\n",
		labelStyle.Render("Range:"),
		v.renderTrendIndicator(rangeTrend),
	)

	// Recent activity
	recent := v.history[0]
	oldest := v.history[len(v.history)-1]
	timespan := recent.UpdatedAt.Sub(oldest.UpdatedAt)

	content += fmt.Sprintf("%s\n%s\n\n",
		labelStyle.Render("Timespan:"),
		valueStyle.Render(formatDuration(timespan)),
	)

	// Data points
	content += fmt.Sprintf("%s\n%s",
		labelStyle.Render("Data Points:"),
		valueStyle.Render(fmt.Sprintf("%d states", len(v.history))),
	)

	return sectionStyle.Width(35).Render("📈 Trends\n\n" + content)
}

func (v *HealthView) renderDiagnostics(state *model.VehicleState, sectionStyle, labelStyle, valueStyle lipgloss.Style) string {
	content := ""

	// Temperature status
	if state.CabinTemp != nil || state.ExteriorTemp != nil {
		content += labelStyle.Render("🌡️  Temperature") + "\n"
		if state.CabinTemp != nil {
			content += fmt.Sprintf("   Cabin: %s\n",
				valueStyle.Render(fmt.Sprintf("%.1f°F", *state.CabinTemp)),
			)
		}
		if state.ExteriorTemp != nil {
			content += fmt.Sprintf("   Exterior: %s\n",
				valueStyle.Render(fmt.Sprintf("%.1f°F", *state.ExteriorTemp)),
			)
		}
		content += "\n"
	}

	// Closure status
	content += labelStyle.Render("🚪 Closures") + "\n"
	content += v.renderClosureStatus("   Doors", state.Doors, valueStyle)
	content += v.renderClosureStatus("   Windows", state.Windows, valueStyle)

	if state.Frunk != model.ClosureStatusUnknown {
		frunkStatus := "closed"
		frunkColor := lipgloss.Color("#00ff00")
		if state.Frunk == model.ClosureStatusOpen {
			frunkStatus = "open"
			frunkColor = lipgloss.Color("#ff0000")
		}
		content += fmt.Sprintf("   Frunk: %s\n",
			valueStyle.Foreground(frunkColor).Render(frunkStatus),
		)
	}

	if state.Liftgate != model.ClosureStatusUnknown {
		liftgateStatus := "closed"
		liftgateColor := lipgloss.Color("#00ff00")
		if state.Liftgate == model.ClosureStatusOpen {
			liftgateStatus = "open"
			liftgateColor = lipgloss.Color("#ff0000")
		}
		content += fmt.Sprintf("   Liftgate: %s\n",
			valueStyle.Foreground(liftgateColor).Render(liftgateStatus),
		)
	}

	// Tire pressures (if available and meaningful)
	if state.TirePressures.FrontLeft > 0 || state.TirePressures.FrontRight > 0 ||
		state.TirePressures.RearLeft > 0 || state.TirePressures.RearRight > 0 {
		content += "\n" + labelStyle.Render("🛞 Tires") + "\n"

		if state.TirePressures.FrontLeft > 0 {
			content += fmt.Sprintf("   FL: %s  FR: %s\n",
				v.formatTirePressure(state.TirePressures.FrontLeft, valueStyle),
				v.formatTirePressure(state.TirePressures.FrontRight, valueStyle),
			)
			content += fmt.Sprintf("   RL: %s  RR: %s\n",
				v.formatTirePressure(state.TirePressures.RearLeft, valueStyle),
				v.formatTirePressure(state.TirePressures.RearRight, valueStyle),
			)
		}
	}

	return sectionStyle.Width(72).Render("🔧 Diagnostics\n\n" + content)
}

func (v *HealthView) renderClosureStatus(label string, closures model.Closures, valueStyle lipgloss.Style) string {
	if closures.AllClosed() {
		return fmt.Sprintf("%s: %s\n",
			label,
			valueStyle.Foreground(lipgloss.Color("#00ff00")).Render("all closed"),
		)
	}

	var openList []string
	if closures.FrontLeft == model.ClosureStatusOpen {
		openList = append(openList, "FL")
	}
	if closures.FrontRight == model.ClosureStatusOpen {
		openList = append(openList, "FR")
	}
	if closures.RearLeft == model.ClosureStatusOpen {
		openList = append(openList, "RL")
	}
	if closures.RearRight == model.ClosureStatusOpen {
		openList = append(openList, "RR")
	}

	if len(openList) > 0 {
		return fmt.Sprintf("%s: %s\n",
			label,
			valueStyle.Foreground(lipgloss.Color("#ff0000")).Render(strings.Join(openList, ", ")+" open"),
		)
	}

	return fmt.Sprintf("%s: %s\n", label, "unknown")
}

func (v *HealthView) formatTirePressure(pressure float64, valueStyle lipgloss.Style) string {
	color := lipgloss.Color("#00ff00")
	if pressure < 30 {
		color = lipgloss.Color("#ff0000")
	} else if pressure < 35 {
		color = lipgloss.Color("#ffff00")
	}

	return valueStyle.Foreground(color).Render(fmt.Sprintf("%.1f PSI", pressure))
}

func (v *HealthView) calculateTrend(metric string) float64 {
	if len(v.history) < 2 {
		return 0
	}

	// Compare most recent with oldest
	recent := v.history[0]
	oldest := v.history[len(v.history)-1]

	switch metric {
	case "battery":
		return recent.BatteryLevel - oldest.BatteryLevel
	case "range":
		return recent.RangeEstimate - oldest.RangeEstimate
	default:
		return 0
	}
}

func (v *HealthView) renderTrendIndicator(change float64) string {
	switch {
	case change > 5:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render(fmt.Sprintf("↑ +%.1f (increasing)", change))
	case change < -5:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render(fmt.Sprintf("↓ %.1f (decreasing)", change))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("→ stable")
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}

	if d < time.Hour {
		minutes := int(d.Minutes())
		return fmt.Sprintf("%d min ago", minutes)
	}

	if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm ago", hours, minutes)
		}
		return fmt.Sprintf("%dh ago", hours)
	}

	days := int(d.Hours() / 24)
	return fmt.Sprintf("%d days ago", days)
}
