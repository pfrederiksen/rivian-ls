package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/model"
)

func TestNewChartsView(t *testing.T) {
	view := NewChartsView(nil, "test-vehicle-id")

	if view == nil {
		t.Fatal("NewChartsView() returned nil")
	}
	if view.vehicleID != "test-vehicle-id" {
		t.Errorf("NewChartsView() vehicleID = %q, want %q", view.vehicleID, "test-vehicle-id")
	}
	if view.selectedMetric != MetricBattery {
		t.Errorf("NewChartsView() default metric = %v, want %v", view.selectedMetric, MetricBattery)
	}
	if view.timeRange != Range24Hours {
		t.Errorf("NewChartsView() default timeRange = %v, want %v", view.timeRange, Range24Hours)
	}
}

func TestChartsView_NextMetric(t *testing.T) {
	view := &ChartsView{
		selectedMetric: MetricBattery,
	}

	// Cycle through all metrics
	expectedOrder := []ChartMetric{
		MetricRange,      // 0 + 1 = 1
		MetricChargingRate, // 1 + 1 = 2
		MetricTemperature,  // 2 + 1 = 3
		MetricEfficiency,   // 3 + 1 = 4
		MetricBattery,      // 4 + 1 = 5 % 5 = 0
	}

	for i, expected := range expectedOrder {
		view.NextMetric()
		if view.selectedMetric != expected {
			t.Errorf("After NextMetric() call %d: got metric %v, want %v", i+1, view.selectedMetric, expected)
		}
	}
}

func TestChartsView_PrevMetric(t *testing.T) {
	view := &ChartsView{
		selectedMetric: MetricBattery,
	}

	// Going backwards from 0 should wrap to 4
	view.PrevMetric()
	if view.selectedMetric != MetricEfficiency {
		t.Errorf("PrevMetric() from Battery = %v, want %v", view.selectedMetric, MetricEfficiency)
	}

	// Then to 3
	view.PrevMetric()
	if view.selectedMetric != MetricTemperature {
		t.Errorf("PrevMetric() from Efficiency = %v, want %v", view.selectedMetric, MetricTemperature)
	}
}

func TestChartsView_NextTimeRange(t *testing.T) {
	view := &ChartsView{
		timeRange: Range24Hours,
	}

	// Cycle through all time ranges
	view.NextTimeRange()
	if view.timeRange != Range7Days {
		t.Errorf("NextTimeRange() = %v, want %v", view.timeRange, Range7Days)
	}

	view.NextTimeRange()
	if view.timeRange != Range30Days {
		t.Errorf("NextTimeRange() = %v, want %v", view.timeRange, Range30Days)
	}

	// Should wrap back to 24 hours
	view.NextTimeRange()
	if view.timeRange != Range24Hours {
		t.Errorf("NextTimeRange() = %v, want %v", view.timeRange, Range24Hours)
	}
}

func TestChartsView_RenderTitle(t *testing.T) {
	tests := []struct {
		metric    ChartMetric
		timeRange TimeRange
		expected  string
	}{
		{MetricBattery, Range24Hours, "📊 Battery Level (Last 24 Hours)"},
		{MetricRange, Range7Days, "📊 Range Estimate (Last 7 Days)"},
		{MetricChargingRate, Range30Days, "📊 Charging Rate (Last 30 Days)"},
		{MetricTemperature, Range24Hours, "📊 Cabin Temperature (Last 24 Hours)"},
		{MetricEfficiency, Range7Days, "📊 Energy Efficiency (Last 7 Days)"},
	}

	for _, tt := range tests {
		view := &ChartsView{
			selectedMetric: tt.metric,
			timeRange:      tt.timeRange,
		}

		got := view.renderTitle()
		if got != tt.expected {
			t.Errorf("renderTitle() = %q, want %q", got, tt.expected)
		}
	}
}

func TestChartsView_CalculateMinMax(t *testing.T) {
	now := time.Now()
	temp70 := 70.0
	temp65 := 65.0
	temp75 := 75.0
	charge10 := 10.0
	charge50 := 50.0
	charge0 := 0.0

	tests := []struct {
		name          string
		metric        ChartMetric
		history       []*model.VehicleState
		expectedMin   float64
		expectedMax   float64
	}{
		{
			name:   "battery levels",
			metric: MetricBattery,
			history: []*model.VehicleState{
				{BatteryLevel: 85.0, UpdatedAt: now},
				{BatteryLevel: 75.0, UpdatedAt: now},
				{BatteryLevel: 90.0, UpdatedAt: now},
			},
			expectedMin: 75.0,
			expectedMax: 90.0,
		},
		{
			name:   "range estimates",
			metric: MetricRange,
			history: []*model.VehicleState{
				{RangeEstimate: 200.0, UpdatedAt: now},
				{RangeEstimate: 150.0, UpdatedAt: now},
				{RangeEstimate: 250.0, UpdatedAt: now},
			},
			expectedMin: 150.0,
			expectedMax: 250.0,
		},
		{
			name:   "charging rate with zeros",
			metric: MetricChargingRate,
			history: []*model.VehicleState{
				{ChargingRate: &charge10, UpdatedAt: now},
				{ChargingRate: &charge0, UpdatedAt: now},
				{ChargingRate: &charge50, UpdatedAt: now},
			},
			expectedMin: 0.0,
			expectedMax: 50.0,
		},
		{
			name:   "temperature with nulls",
			metric: MetricTemperature,
			history: []*model.VehicleState{
				{CabinTemp: &temp70, UpdatedAt: now},
				{CabinTemp: nil, UpdatedAt: now}, // Should be skipped
				{CabinTemp: &temp65, UpdatedAt: now},
				{CabinTemp: &temp75, UpdatedAt: now},
			},
			expectedMin: 65.0,
			expectedMax: 75.0,
		},
		{
			name:        "empty history",
			metric:      MetricBattery,
			history:     []*model.VehicleState{},
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
		{
			name:   "single data point",
			metric: MetricBattery,
			history: []*model.VehicleState{
				{BatteryLevel: 80.0, UpdatedAt: now},
			},
			expectedMin: 80.0,
			expectedMax: 80.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := &ChartsView{
				history: tt.history,
			}

			min, max := view.calculateMinMax(tt.metric)

			if min != tt.expectedMin {
				t.Errorf("calculateMinMax() min = %v, want %v", min, tt.expectedMin)
			}
			if max != tt.expectedMax {
				t.Errorf("calculateMinMax() max = %v, want %v", max, tt.expectedMax)
			}
		})
	}
}

func TestChartsView_RenderNoData(t *testing.T) {
	view := &ChartsView{}
	output := view.renderNoData()

	// Check for expected content
	if !strings.Contains(output, "No historical data available") {
		t.Error("renderNoData() should mention no data available")
	}
	if !strings.Contains(output, "Charts will populate") {
		t.Error("renderNoData() should mention charts will populate")
	}
}

func TestChartsView_RenderSingleDataPoint(t *testing.T) {
	view := &ChartsView{}
	output := view.renderSingleDataPoint("Battery Level", 85.5, "%")

	// Check for expected content
	if !strings.Contains(output, "Battery Level") {
		t.Error("renderSingleDataPoint() should include metric name")
	}
	if !strings.Contains(output, "85.5%") {
		t.Error("renderSingleDataPoint() should include value with unit")
	}
	if !strings.Contains(output, "Need at least 2 data points") {
		t.Error("renderSingleDataPoint() should mention need for more data")
	}
}

func TestChartsView_GenerateTimeLabels(t *testing.T) {
	oldest := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	newest := time.Date(2024, 1, 2, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name          string
		timeRange     TimeRange
		checkContains []string
	}{
		{
			name:          "24 hours shows time",
			timeRange:     Range24Hours,
			checkContains: []string{"AM", "PM"}, // Should contain time format
		},
		{
			name:          "7 days shows dates",
			timeRange:     Range7Days,
			checkContains: []string{"Jan"}, // Should contain month
		},
		{
			name:          "30 days shows dates",
			timeRange:     Range30Days,
			checkContains: []string{"Jan"}, // Should contain month
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := &ChartsView{
				timeRange: tt.timeRange,
				history: []*model.VehicleState{
					{UpdatedAt: oldest},
					{UpdatedAt: newest},
				},
			}

			output := view.generateTimeLabels()

			for _, expected := range tt.checkContains {
				if !strings.Contains(output, expected) {
					t.Errorf("generateTimeLabels() missing %q in output: %q", expected, output)
				}
			}
		})
	}
}

func TestChartsView_GenerateTimeLabels_EmptyHistory(t *testing.T) {
	view := &ChartsView{
		history: []*model.VehicleState{},
	}

	output := view.generateTimeLabels()
	if output != "" {
		t.Errorf("generateTimeLabels() with empty history = %q, want empty string", output)
	}
}

func TestChartsView_RenderEfficiencyChart_Calculation(t *testing.T) {
	// Test the efficiency calculation logic
	now := time.Now()
	view := &ChartsView{
		history: []*model.VehicleState{
			// Oldest: 80% battery, 200 miles
			{BatteryLevel: 80.0, RangeEstimate: 200.0, UpdatedAt: now.Add(-2 * time.Hour)},
			// Middle: 70% battery, 175 miles (10% used = 14 kWh, 25 miles driven = 1.79 mi/kWh)
			{BatteryLevel: 70.0, RangeEstimate: 175.0, UpdatedAt: now.Add(-1 * time.Hour)},
			// Newest: 60% battery, 150 miles (10% used = 14 kWh, 25 miles driven = 1.79 mi/kWh)
			{BatteryLevel: 60.0, RangeEstimate: 150.0, UpdatedAt: now},
		},
		selectedMetric: MetricEfficiency,
	}

	// This tests that the chart doesn't crash with valid data
	// Actual calculation: batteryDelta * 1.40 = energyUsed
	// efficiency = rangeDelta / energyUsed
	output := view.renderEfficiencyChart(80, 20)

	// Should not show "no data" message
	if strings.Contains(output, "No historical data") {
		t.Error("renderEfficiencyChart() with valid data should not show no data message")
	}

	// Should not show single data point message
	if strings.Contains(output, "Need at least 2 data points") {
		t.Error("renderEfficiencyChart() with multiple points should not show single data point message")
	}
}

func TestChartsView_RenderEfficiencyChart_InsufficientData(t *testing.T) {
	now := time.Now()
	view := &ChartsView{
		history: []*model.VehicleState{
			// Only one state - can't calculate efficiency
			{BatteryLevel: 80.0, RangeEstimate: 200.0, UpdatedAt: now},
		},
		selectedMetric: MetricEfficiency,
	}

	output := view.renderEfficiencyChart(80, 20)

	// Should show "no data" or "insufficient" message
	if !strings.Contains(output, "No historical data") && !strings.Contains(output, "Not enough data") {
		t.Error("renderEfficiencyChart() with single state should show insufficient data message")
	}
}

func TestChartsView_MetricSwitchingInvalidatesCache(t *testing.T) {
	now := time.Now()
	view := &ChartsView{
		history: []*model.VehicleState{
			{BatteryLevel: 80.0, UpdatedAt: now},
		},
		selectedMetric: MetricBattery,
	}

	// Switching metric should invalidate cache
	view.NextMetric()
	if view.history != nil {
		t.Error("NextMetric() should invalidate history cache")
	}

	// Restore history for next test
	view.history = []*model.VehicleState{
		{BatteryLevel: 80.0, UpdatedAt: now},
	}

	view.PrevMetric()
	if view.history != nil {
		t.Error("PrevMetric() should invalidate history cache")
	}
}

func TestChartsView_TimeRangeSwitchingInvalidatesCache(t *testing.T) {
	now := time.Now()
	view := &ChartsView{
		history: []*model.VehicleState{
			{BatteryLevel: 80.0, UpdatedAt: now},
		},
		timeRange: Range24Hours,
	}

	// Switching time range should invalidate cache
	view.NextTimeRange()
	if view.history != nil {
		t.Error("NextTimeRange() should invalidate history cache")
	}
}

func TestChartsView_RenderSimpleChart(t *testing.T) {
	now := time.Now()
	view := &ChartsView{
		history: []*model.VehicleState{
			{BatteryLevel: 80.0, UpdatedAt: now.Add(-2 * time.Hour)},
			{BatteryLevel: 75.0, UpdatedAt: now.Add(-1 * time.Hour)},
			{BatteryLevel: 70.0, UpdatedAt: now},
		},
		timeRange: Range24Hours,
	}

	data := []float64{80.0, 75.0, 70.0}
	output := view.renderSimpleChart(data, "Test Metric", "units", 80, 20)

	// Should contain chart elements
	if !strings.Contains(output, "│") && !strings.Contains(output, "┤") {
		t.Error("renderSimpleChart() should contain chart box drawing characters")
	}
}

func TestChartsView_RenderSimpleChart_NoData(t *testing.T) {
	view := &ChartsView{
		history: []*model.VehicleState{},
	}

	data := []float64{}
	output := view.renderSimpleChart(data, "Test Metric", "units", 80, 20)

	// Should show no data message
	if !strings.Contains(output, "No historical data") {
		t.Error("renderSimpleChart() with no data should show no data message")
	}
}

func TestChartsView_RenderSimpleChart_SinglePoint(t *testing.T) {
	now := time.Now()
	view := &ChartsView{
		history: []*model.VehicleState{
			{BatteryLevel: 80.0, UpdatedAt: now},
		},
	}

	data := []float64{80.0}
	output := view.renderSimpleChart(data, "Test Metric", "units", 80, 20)

	// Should show single data point message
	if !strings.Contains(output, "Need at least 2 data points") {
		t.Error("renderSimpleChart() with single point should show single data point message")
	}
	if !strings.Contains(output, "Test Metric") {
		t.Error("renderSimpleChart() should include metric name")
	}
}
