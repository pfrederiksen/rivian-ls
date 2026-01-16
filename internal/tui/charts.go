package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/store"
)

// ChartMetric represents the type of metric to display
type ChartMetric int

const (
	MetricBattery ChartMetric = iota
	MetricRange
	MetricChargingRate
	MetricTemperature
	MetricEfficiency
)

// TimeRange represents the time range for the chart
type TimeRange int

const (
	Range24Hours TimeRange = iota
	Range7Days
	Range30Days
)

// ChartsView handles the charts display
type ChartsView struct {
	store          *store.Store
	vehicleID      string
	history        []*model.VehicleState
	selectedMetric ChartMetric
	timeRange      TimeRange
	lastLoad       time.Time
}

// NewChartsView creates a new charts view
func NewChartsView(store *store.Store, vehicleID string) *ChartsView {
	return &ChartsView{
		store:          store,
		vehicleID:      vehicleID,
		selectedMetric: MetricBattery,
		timeRange:      Range24Hours,
	}
}

// NextMetric switches to the next metric
func (v *ChartsView) NextMetric() {
	v.selectedMetric = (v.selectedMetric + 1) % 5 // 5 metrics total
	// Invalidate cache to reload data
	v.history = nil
}

// PrevMetric switches to the previous metric
func (v *ChartsView) PrevMetric() {
	if v.selectedMetric == 0 {
		v.selectedMetric = 4 // Wrap to last metric
	} else {
		v.selectedMetric--
	}
	// Invalidate cache to reload data
	v.history = nil
}

// NextTimeRange cycles to the next time range
func (v *ChartsView) NextTimeRange() {
	v.timeRange = (v.timeRange + 1) % 3 // 3 time ranges total
	// Invalidate cache to reload data
	v.history = nil
}

// Render renders the charts view
func (v *ChartsView) Render(state *model.VehicleState, width, height int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ffff")).
		Bold(true).
		MarginTop(1).
		MarginBottom(1)

	// Load history if not cached or if stale
	if v.history == nil || time.Since(v.lastLoad) > 30*time.Second {
		v.loadHistory()
	}

	// Render title with metric and time range
	title := v.renderTitle()

	// Check if we have enough data
	if len(v.history) == 0 {
		return titleStyle.Render("📊 Charts") + "\n\n" + v.renderNoData()
	}

	// Render chart based on selected metric
	var chart string
	switch v.selectedMetric {
	case MetricBattery:
		chart = v.renderBatteryChart(width-4, height-15)
	case MetricRange:
		chart = v.renderRangeChart(width-4, height-15)
	case MetricChargingRate:
		chart = v.renderChargingRateChart(width-4, height-15)
	case MetricTemperature:
		chart = v.renderTemperatureChart(width-4, height-15)
	case MetricEfficiency:
		chart = v.renderEfficiencyChart(width-4, height-15)
	default:
		chart = "Unknown metric"
	}

	// Render statistics
	stats := v.renderStats(state)

	return titleStyle.Render(title) + "\n" + chart + "\n\n" + stats
}

// renderTitle renders the chart title
func (v *ChartsView) renderTitle() string {
	var metricName string
	switch v.selectedMetric {
	case MetricBattery:
		metricName = "Battery Level"
	case MetricRange:
		metricName = "Range Estimate"
	case MetricChargingRate:
		metricName = "Charging Rate"
	case MetricTemperature:
		metricName = "Cabin Temperature"
	case MetricEfficiency:
		metricName = "Energy Efficiency"
	default:
		metricName = "Unknown"
	}

	var timeRangeName string
	switch v.timeRange {
	case Range24Hours:
		timeRangeName = "Last 24 Hours"
	case Range7Days:
		timeRangeName = "Last 7 Days"
	case Range30Days:
		timeRangeName = "Last 30 Days"
	default:
		timeRangeName = "Unknown"
	}

	return fmt.Sprintf("📊 %s (%s)", metricName, timeRangeName)
}

// renderNoData renders a message when no data is available
func (v *ChartsView) renderNoData() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Align(lipgloss.Center).
		Padding(2)

	return style.Render("📊 No historical data available yet\n\nCharts will populate as data is collected")
}

// loadHistory loads historical data from the store
func (v *ChartsView) loadHistory() {
	if v.store == nil {
		return
	}

	ctx := context.Background()

	// Calculate time range
	var since time.Time
	var limit int
	switch v.timeRange {
	case Range24Hours:
		since = time.Now().Add(-24 * time.Hour)
		limit = 100
	case Range7Days:
		since = time.Now().Add(-7 * 24 * time.Hour)
		limit = 200
	case Range30Days:
		since = time.Now().Add(-30 * 24 * time.Hour)
		limit = 300
	default:
		since = time.Now().Add(-24 * time.Hour)
		limit = 100
	}

	// Load from store
	history, err := v.store.GetStateHistory(ctx, v.vehicleID, since, limit)
	if err == nil {
		v.history = history
		v.lastLoad = time.Now()
	}
}

// renderBatteryChart renders the battery level chart
func (v *ChartsView) renderBatteryChart(width, height int) string {
	if len(v.history) == 0 {
		return v.renderNoData()
	}

	// Extract battery data (reverse order - oldest to newest for chart)
	data := make([]float64, 0, len(v.history))
	for i := len(v.history) - 1; i >= 0; i-- {
		data = append(data, v.history[i].BatteryLevel)
	}

	// Handle single data point
	if len(data) == 1 {
		return v.renderSingleDataPoint("Battery Level", data[0], "%")
	}

	// Render chart
	graph := asciigraph.Plot(
		data,
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Caption(v.generateTimeLabels()),
	)

	// Style the graph
	graphStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(1).
		Width(width + 4) // Account for border and padding

	return graphStyle.Render(graph)
}

// renderRangeChart renders the range estimate chart
func (v *ChartsView) renderRangeChart(width, height int) string {
	if len(v.history) == 0 {
		return v.renderNoData()
	}

	// Extract range data (reverse order - oldest to newest for chart)
	data := make([]float64, 0, len(v.history))
	for i := len(v.history) - 1; i >= 0; i-- {
		data = append(data, v.history[i].RangeEstimate)
	}

	// Handle single data point
	if len(data) == 1 {
		return v.renderSingleDataPoint("Range Estimate", data[0], "mi")
	}

	// Render chart
	graph := asciigraph.Plot(
		data,
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Caption(v.generateTimeLabels()),
	)

	// Style the graph
	graphStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(1).
		Width(width + 4) // Account for border and padding

	return graphStyle.Render(graph)
}

// renderChargingRateChart renders the charging rate chart
func (v *ChartsView) renderChargingRateChart(width, height int) string {
	if len(v.history) == 0 {
		return v.renderNoData()
	}

	// Extract charging rate data (reverse order - oldest to newest for chart)
	// Filter for when actually charging
	data := make([]float64, 0)
	for i := len(v.history) - 1; i >= 0; i-- {
		// Only include data points where the vehicle is charging
		if v.history[i].ChargingRate != nil && *v.history[i].ChargingRate > 0 {
			data = append(data, *v.history[i].ChargingRate)
		} else {
			// Fill with 0 for non-charging periods
			data = append(data, 0)
		}
	}

	// Handle single data point
	if len(data) == 1 {
		return v.renderSingleDataPoint("Charging Rate", data[0], "kW")
	}

	// Render chart
	graph := asciigraph.Plot(
		data,
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Caption(v.generateTimeLabels()),
	)

	// Style the graph
	graphStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(1).
		Width(width + 4)

	return graphStyle.Render(graph)
}

// renderTemperatureChart renders the cabin temperature chart
func (v *ChartsView) renderTemperatureChart(width, height int) string {
	if len(v.history) == 0 {
		return v.renderNoData()
	}

	// Extract temperature data (reverse order - oldest to newest for chart)
	data := make([]float64, 0)
	for i := len(v.history) - 1; i >= 0; i-- {
		if v.history[i].CabinTemp != nil {
			data = append(data, *v.history[i].CabinTemp)
		}
	}

	// Handle insufficient data
	if len(data) == 0 {
		return v.renderNoData()
	}
	if len(data) == 1 {
		return v.renderSingleDataPoint("Cabin Temperature", data[0], "°F")
	}

	// Render chart
	graph := asciigraph.Plot(
		data,
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Caption(v.generateTimeLabels()),
	)

	// Style the graph
	graphStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(1).
		Width(width + 4)

	return graphStyle.Render(graph)
}

// renderEfficiencyChart renders the energy efficiency chart (mi/kWh)
func (v *ChartsView) renderEfficiencyChart(width, height int) string {
	if len(v.history) < 2 {
		return v.renderNoData()
	}

	// Calculate efficiency for each pair of points
	data := make([]float64, 0)
	for i := len(v.history) - 1; i > 0; i-- {
		curr := v.history[i-1]
		prev := v.history[i]

		// Calculate efficiency: change in range / change in battery %
		batteryDelta := prev.BatteryLevel - curr.BatteryLevel
		rangeDelta := prev.RangeEstimate - curr.RangeEstimate

		// Only calculate if battery changed
		if batteryDelta > 0.1 {
			// Estimate energy used (assume ~140kWh capacity for calculation)
			energyUsed := batteryDelta * 1.40 // kWh
			if energyUsed > 0 {
				efficiency := rangeDelta / energyUsed
				// Clamp to reasonable values
				if efficiency > 0 && efficiency < 10 {
					data = append(data, efficiency)
				}
			}
		}
	}

	// Handle insufficient data
	if len(data) == 0 {
		noDataStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Align(lipgloss.Center).
			Padding(2)
		return noDataStyle.Render("📊 Not enough data to calculate efficiency\n\nNeed battery and range changes over time")
	}
	if len(data) == 1 {
		return v.renderSingleDataPoint("Efficiency", data[0], "mi/kWh")
	}

	// Render chart
	graph := asciigraph.Plot(
		data,
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Caption(v.generateTimeLabels()),
	)

	// Style the graph
	graphStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(1).
		Width(width + 4)

	return graphStyle.Render(graph)
}

// renderSingleDataPoint renders a display for when there's only one data point
func (v *ChartsView) renderSingleDataPoint(metric string, value float64, unit string) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(2).
		Align(lipgloss.Center)

	return style.Render(fmt.Sprintf("%s: %.1f%s\n\nNeed at least 2 data points to display a chart", metric, value, unit))
}

// generateTimeLabels generates time labels for the X-axis
func (v *ChartsView) generateTimeLabels() string {
	if len(v.history) == 0 {
		return ""
	}

	// Get oldest and newest timestamps
	oldest := v.history[len(v.history)-1].UpdatedAt
	newest := v.history[0].UpdatedAt

	// Format based on time range
	switch v.timeRange {
	case Range24Hours:
		// Show hour labels
		return fmt.Sprintf("%s → %s",
			oldest.Format("3:04 PM"),
			newest.Format("3:04 PM"))
	case Range7Days:
		// Show day labels
		return fmt.Sprintf("%s → %s",
			oldest.Format("Jan 2"),
			newest.Format("Jan 2"))
	case Range30Days:
		// Show date labels
		return fmt.Sprintf("%s → %s",
			oldest.Format("Jan 2"),
			newest.Format("Jan 2"))
	default:
		return ""
	}
}

// renderStats renders statistics about the data
func (v *ChartsView) renderStats(state *model.VehicleState) string {
	if len(v.history) == 0 {
		return ""
	}

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true)

	// Calculate stats based on metric
	var current, min, max, change float64
	var unit string

	switch v.selectedMetric {
	case MetricBattery:
		current = state.BatteryLevel
		min, max = v.calculateMinMax(MetricBattery)
		change = v.history[0].BatteryLevel - v.history[len(v.history)-1].BatteryLevel
		unit = "%"
	case MetricRange:
		current = state.RangeEstimate
		min, max = v.calculateMinMax(MetricRange)
		change = v.history[0].RangeEstimate - v.history[len(v.history)-1].RangeEstimate
		unit = " mi"
	case MetricChargingRate:
		if state.ChargingRate != nil {
			current = *state.ChargingRate
		}
		min, max = v.calculateMinMax(MetricChargingRate)
		if len(v.history) > 0 && v.history[0].ChargingRate != nil && v.history[len(v.history)-1].ChargingRate != nil {
			change = *v.history[0].ChargingRate - *v.history[len(v.history)-1].ChargingRate
		}
		unit = " kW"
	case MetricTemperature:
		if state.CabinTemp != nil {
			current = *state.CabinTemp
		}
		min, max = v.calculateMinMax(MetricTemperature)
		if len(v.history) > 0 && v.history[0].CabinTemp != nil && v.history[len(v.history)-1].CabinTemp != nil {
			change = *v.history[0].CabinTemp - *v.history[len(v.history)-1].CabinTemp
		}
		unit = "°F"
	case MetricEfficiency:
		// For efficiency, calculate average from history
		data := make([]float64, 0)
		for i := len(v.history) - 1; i > 0; i-- {
			curr := v.history[i-1]
			prev := v.history[i]
			batteryDelta := prev.BatteryLevel - curr.BatteryLevel
			rangeDelta := prev.RangeEstimate - curr.RangeEstimate
			if batteryDelta > 0.1 {
				energyUsed := batteryDelta * 1.40
				if energyUsed > 0 {
					efficiency := rangeDelta / energyUsed
					if efficiency > 0 && efficiency < 10 {
						data = append(data, efficiency)
					}
				}
			}
		}
		if len(data) > 0 {
			// Calculate average as current
			sum := 0.0
			for _, v := range data {
				sum += v
			}
			current = sum / float64(len(data))
			// Find min/max from calculated data
			min = data[0]
			max = data[0]
			for _, v := range data {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}
			// Change is difference between most recent and oldest efficiency
			change = data[len(data)-1] - data[0]
		}
		unit = " mi/kWh"
	}

	// Format change with sign
	changeStr := fmt.Sprintf("%+.1f%s", change, unit)
	if change > 0 {
		changeStr = valueStyle.Foreground(lipgloss.Color("#00ff00")).Render(changeStr)
	} else if change < 0 {
		changeStr = valueStyle.Foreground(lipgloss.Color("#ff0000")).Render(changeStr)
	} else {
		changeStr = valueStyle.Render(changeStr)
	}

	stats := fmt.Sprintf("%s %s  │  %s %.1f%s  │  %s %.1f%s  │  %s %s",
		labelStyle.Render("Current:"),
		valueStyle.Render(fmt.Sprintf("%.1f%s", current, unit)),
		labelStyle.Render("Min:"),
		min,
		unit,
		labelStyle.Render("Max:"),
		max,
		unit,
		labelStyle.Render("Change:"),
		changeStr,
	)

	statStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f5fff")).
		Padding(0, 1)

	return statStyle.Render(stats)
}

// calculateMinMax calculates min and max values for a metric
func (v *ChartsView) calculateMinMax(metric ChartMetric) (min, max float64) {
	if len(v.history) == 0 {
		return 0, 0
	}

	// Initialize with first value
	var firstValue float64
	switch metric {
	case MetricBattery:
		firstValue = v.history[0].BatteryLevel
	case MetricRange:
		firstValue = v.history[0].RangeEstimate
	case MetricChargingRate:
		if v.history[0].ChargingRate != nil {
			firstValue = *v.history[0].ChargingRate
		}
	case MetricTemperature:
		if v.history[0].CabinTemp != nil {
			firstValue = *v.history[0].CabinTemp
		}
	}

	min = firstValue
	max = firstValue

	// Find min/max
	for _, state := range v.history {
		var value float64
		switch metric {
		case MetricBattery:
			value = state.BatteryLevel
		case MetricRange:
			value = state.RangeEstimate
		case MetricChargingRate:
			if state.ChargingRate != nil {
				value = *state.ChargingRate
			} else {
				continue // Skip nil values
			}
		case MetricTemperature:
			if state.CabinTemp != nil {
				value = *state.CabinTemp
			} else {
				continue // Skip nil values
			}
		}

		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	return min, max
}
