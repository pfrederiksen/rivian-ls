package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

// VehicleMenu handles the vehicle selection overlay
type VehicleMenu struct {
	vehicles      []rivian.Vehicle
	selectedIndex int
	states        map[string]*model.VehicleState // For displaying battery %
}

// NewVehicleMenu creates a new vehicle menu
func NewVehicleMenu(vehicles []rivian.Vehicle, currentIndex int, states map[string]*model.VehicleState) *VehicleMenu {
	return &VehicleMenu{
		vehicles:      vehicles,
		selectedIndex: currentIndex,
		states:        states,
	}
}

// HandleKey processes keyboard input for the menu
// Returns: (selected index, done selecting)
func (m *VehicleMenu) HandleKey(key string) (int, bool) {
	switch key {
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
		return m.selectedIndex, false

	case "down", "j":
		if m.selectedIndex < len(m.vehicles)-1 {
			m.selectedIndex++
		}
		return m.selectedIndex, false

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Direct selection by number
		index := int(key[0] - '1')
		if index >= 0 && index < len(m.vehicles) {
			m.selectedIndex = index
		}
		return m.selectedIndex, false

	case "enter":
		// Confirm selection
		return m.selectedIndex, true

	case "esc", "q":
		// Cancel - return -1 to indicate no change
		return -1, true

	default:
		return m.selectedIndex, false
	}
}

// Render renders the vehicle selection menu as an overlay
func (m *VehicleMenu) Render(width, height int) string {
	// Styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(1, 2).
		Width(width - 20) // Leave margin

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ffff")).
		Bold(true).
		Align(lipgloss.Center)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ffff")).
		Bold(true)

	unselectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Align(lipgloss.Center).
		MarginTop(1)

	// Build content
	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("Select Vehicle"))
	content.WriteString("\n\n")

	// Vehicle list
	for i, vehicle := range m.vehicles {
		var line strings.Builder

		// Selection indicator
		if i == m.selectedIndex {
			line.WriteString("→ ")
		} else {
			line.WriteString("  ")
		}

		// Number
		line.WriteString(fmt.Sprintf("%d. ", i+1))

		// Vehicle info
		vehicleInfo := fmt.Sprintf("%s %q", vehicle.Model, vehicle.Name)
		if vehicle.Model == "" {
			vehicleInfo = fmt.Sprintf("%q", vehicle.Name)
		}
		if vehicle.Name == "" {
			vehicleInfo = vehicle.Model
			if vehicleInfo == "" {
				vehicleInfo = fmt.Sprintf("VIN: ...%s", vehicle.VIN[len(vehicle.VIN)-6:])
			}
		}

		// Battery level and status
		batteryStr := "[--]"
		statusIcon := "🟡" // Unknown
		if state, ok := m.states[vehicle.ID]; ok && state != nil {
			batteryStr = fmt.Sprintf("[%.0f%%]", state.BatteryLevel)
			if state.IsOnline {
				statusIcon = "🟢"
			} else {
				statusIcon = "🔴"
			}
		}

		fullLine := fmt.Sprintf("%-35s %s %s", vehicleInfo, batteryStr, statusIcon)

		// Apply style
		if i == m.selectedIndex {
			content.WriteString(selectedStyle.Render(line.String() + fullLine))
		} else {
			content.WriteString(unselectedStyle.Render(line.String() + fullLine))
		}
		content.WriteString("\n")
	}

	// Help text
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("[↑/↓] Navigate  [1-9] Select  [Enter] Confirm  [Esc] Cancel"))

	// Wrap in border
	menu := borderStyle.Render(content.String())

	// Center on screen
	centered := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		menu,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1a1a")),
	)

	return centered
}
