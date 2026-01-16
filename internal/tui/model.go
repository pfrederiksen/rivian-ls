package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
	"github.com/pfrederiksen/rivian-ls/internal/store"
)

// ViewType represents the current active view
type ViewType int

const (
	ViewDashboard ViewType = iota
	ViewCharge
	ViewHealth
	ViewCharts
)

// Model is the main Bubble Tea model for the TUI
type Model struct {
	// Core dependencies
	client  rivian.Client
	store   *store.Store

	// Multi-vehicle state
	vehicles      []rivian.Vehicle                   // All available vehicles
	activeVehicle int                                // Currently selected vehicle index
	vehicleStates map[string]*model.VehicleState     // vehicleID -> cached state
	reducers      map[string]*model.Reducer          // vehicleID -> reducer instance
	wsClients     map[string]*rivian.WebSocketClient // vehicleID -> WebSocket client
	updateChans   map[string]chan *model.VehicleState // vehicleID -> update channel

	// Application state
	currentView ViewType
	state       *model.VehicleState // Current active vehicle's state
	err         error
	loading     bool

	// Subscription state
	ctx    context.Context
	cancel context.CancelFunc

	// Sub-views
	dashboardView *DashboardView
	chargeView    *ChargeView
	healthView    *HealthView
	chartsView    *ChartsView

	// Vehicle menu
	showVehicleMenu bool
	vehicleMenu     *VehicleMenu

	// Terminal dimensions
	width  int
	height int

	// Last update time
	lastUpdate time.Time
}

// NewModel creates a new TUI model with multi-vehicle support
func NewModel(client rivian.Client, store *store.Store, vehicles []rivian.Vehicle, startIndex int) *Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Validate startIndex
	if startIndex < 0 || startIndex >= len(vehicles) {
		startIndex = 0
	}

	// Initialize vehicleID for views
	var vehicleID string
	if len(vehicles) > 0 {
		vehicleID = vehicles[startIndex].ID
	}

	return &Model{
		client:        client,
		store:         store,
		vehicles:      vehicles,
		activeVehicle: startIndex,
		vehicleStates: make(map[string]*model.VehicleState),
		reducers:      make(map[string]*model.Reducer),
		wsClients:     make(map[string]*rivian.WebSocketClient),
		updateChans:   make(map[string]chan *model.VehicleState),
		currentView:   ViewDashboard,
		loading:       true,
		ctx:           ctx,
		cancel:        cancel,
		dashboardView: NewDashboardView(),
		chargeView:    NewChargeView(),
		healthView:    NewHealthView(store, vehicleID),
		chartsView:    NewChartsView(store, vehicleID),
	}
}

// Init initializes the model (Bubble Tea lifecycle method)
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchInitialState(),
		m.subscribeToUpdates(),
		// Note: We don't call waitForUpdates() here because if WebSocket
		// connection fails, nothing will ever be sent to the channel.
		// waitForUpdates() is only called after receiving the first update.
		tea.EnterAltScreen,
	)
}

// Update handles messages and updates the model (Bubble Tea lifecycle method)
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case initialStateMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.state = msg.state
		m.lastUpdate = time.Now()
		return m, nil

	case stateUpdateMsg:
		m.state = msg.state
		m.lastUpdate = time.Now()
		return m, m.waitForUpdates()

	case errMsg:
		m.err = msg.err
		return m, nil

	case wsConnectedMsg:
		// WebSocket connected successfully, start waiting for updates
		return m, m.waitForUpdates()

	default:
		return m, nil
	}
}

// View renders the current view (Bubble Tea lifecycle method)
func (m *Model) View() string {
	if m.loading {
		return m.renderLoading()
	}

	if m.err != nil {
		return m.renderError()
	}

	if m.state == nil {
		return "No vehicle data available"
	}

	// Render header
	header := m.renderHeader()

	// Render current view
	var content string
	switch m.currentView {
	case ViewDashboard:
		content = m.dashboardView.Render(m.state, m.width, m.height-lipgloss.Height(header)-3)
	case ViewCharge:
		content = m.chargeView.Render(m.state, m.width, m.height-lipgloss.Height(header)-3)
	case ViewHealth:
		content = m.healthView.Render(m.state, m.width, m.height-lipgloss.Height(header)-3)
	case ViewCharts:
		content = m.chartsView.Render(m.state, m.width, m.height-lipgloss.Height(header)-3)
	}

	// Render footer with keyboard shortcuts
	footer := m.renderFooter()

	// Build base view
	baseView := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		footer,
	)

	// If vehicle menu is open, overlay it
	if m.showVehicleMenu && m.vehicleMenu != nil {
		// Render menu on top of base view
		menuOverlay := m.vehicleMenu.Render(m.width, m.height)
		// Layer menu over base - this creates the overlay effect
		return menuOverlay
	}

	return baseView
}

// handleKeyPress processes keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If vehicle menu is open, route keys to it
	if m.showVehicleMenu {
		selectedIndex, done := m.vehicleMenu.HandleKey(msg.String())
		if done {
			if selectedIndex >= 0 && selectedIndex != m.activeVehicle {
				// User confirmed a different vehicle
				m.showVehicleMenu = false
				return m, m.switchVehicle(selectedIndex)
			}
			// User canceled or selected same vehicle
			m.showVehicleMenu = false
		}
		return m, nil
	}

	// Normal key handling when menu is closed
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancel()
		return m, tea.Quit

	case "v":
		// Toggle vehicle menu
		if len(m.vehicles) > 0 {
			m.vehicleMenu = NewVehicleMenu(m.vehicles, m.activeVehicle, m.vehicleStates)
			m.showVehicleMenu = true
		}
		return m, nil

	case "1", "d":
		m.currentView = ViewDashboard
		return m, nil

	case "2", "c":
		m.currentView = ViewCharge
		return m, nil

	case "3", "h":
		m.currentView = ViewHealth
		return m, nil

	case "4":
		m.currentView = ViewCharts
		return m, nil

	case "r":
		// Refresh data
		return m, m.fetchInitialState()

	case "left":
		// Switch to previous metric in charts view
		if m.currentView == ViewCharts {
			m.chartsView.PrevMetric()
		}
		return m, nil

	case "right":
		// Switch to next metric in charts view
		if m.currentView == ViewCharts {
			m.chartsView.NextMetric()
		}
		return m, nil

	case "t":
		// Cycle time range in charts view
		if m.currentView == ViewCharts {
			m.chartsView.NextTimeRange()
		}
		return m, nil

	default:
		return m, nil
	}
}

// Messages

type initialStateMsg struct {
	state *model.VehicleState
	err   error
}

type stateUpdateMsg struct {
	state *model.VehicleState
}

type errMsg struct {
	err error
}

type wsConnectedMsg struct{}


// Commands

func (m *Model) fetchInitialState() tea.Cmd {
	return func() tea.Msg {
		// Check if we have vehicles loaded
		if len(m.vehicles) == 0 {
			return initialStateMsg{err: fmt.Errorf("no vehicles available")}
		}

		// Get the active vehicle
		vehicle := m.vehicles[m.activeVehicle]
		vehicleID := vehicle.ID

		// Get or create reducer for this vehicle
		if m.reducers[vehicleID] == nil {
			m.reducers[vehicleID] = model.NewReducer()
		}
		reducer := m.reducers[vehicleID]

		// Check cache first
		if cachedState, ok := m.vehicleStates[vehicleID]; ok {
			return initialStateMsg{state: cachedState}
		}

		// Try to get latest state from API
		rivState, err := m.client.GetVehicleState(m.ctx, vehicleID)
		if err != nil {
			// Fall back to cached data from store
			if m.store != nil {
				states, err := m.store.GetStateHistory(m.ctx, vehicleID, time.Now().Add(-30*24*time.Hour), 1)
				if err == nil && len(states) > 0 {
					m.vehicleStates[vehicleID] = states[0]
					return initialStateMsg{state: states[0]}
				}
			}
			return initialStateMsg{err: fmt.Errorf("failed to fetch vehicle state: %w", err)}
		}

		// Dispatch VehicleListReceived to set identity
		event := model.VehicleListReceived{
			Vehicles:  []rivian.Vehicle{vehicle},
			VehicleID: vehicleID,
		}
		reducer.Dispatch(event)

		// Convert to domain model
		domainState := model.FromRivianVehicleState(rivState)

		// Dispatch VehicleStateReceived
		stateEvent := model.VehicleStateReceived{State: rivState}
		finalState := reducer.Dispatch(stateEvent)

		// Cache the state
		m.vehicleStates[vehicleID] = finalState

		// Save to store (silently fail - not critical for TUI operation)
		if m.store != nil {
			_ = m.store.SaveState(m.ctx, domainState)
		}

		return initialStateMsg{state: finalState}
	}
}

func (m *Model) subscribeToUpdates() tea.Cmd {
	return func() tea.Msg {
		// Check if we have vehicles loaded
		if len(m.vehicles) == 0 {
			// Non-fatal: silently continue without WebSocket
			return nil
		}

		// Get active vehicle ID
		vehicleID := m.vehicles[m.activeVehicle].ID

		// Get HTTP client
		httpClient, ok := m.client.(*rivian.HTTPClient)
		if !ok {
			// Non-fatal: silently continue without WebSocket
			return nil
		}

		// Create session (gets fresh CSRF and app session tokens)
		if err := httpClient.CreateSession(m.ctx); err != nil {
			// Non-fatal: silently continue without WebSocket
			return nil
		}

		// Get credentials for WebSocket
		creds := httpClient.GetCredentials()
		if creds == nil {
			// Non-fatal: silently continue without WebSocket
			return nil
		}

		// Get tokens needed for WebSocket connection
		csrfToken := httpClient.GetCSRFToken()
		appSessionID := httpClient.GetAppSessionID()

		// Create WebSocket client
		wsClient := rivian.NewWebSocketClient(creds, csrfToken, appSessionID)
		m.wsClients[vehicleID] = wsClient

		// Connect
		if err := wsClient.Connect(m.ctx); err != nil {
			// Non-fatal: silently continue without WebSocket
			// The user can still manually refresh with 'r' key
			return nil
		}

		// Get or create update channel for this vehicle
		if m.updateChans[vehicleID] == nil {
			m.updateChans[vehicleID] = make(chan *model.VehicleState, 10)
		}
		updateChan := m.updateChans[vehicleID]

		// Get or create reducer for this vehicle
		if m.reducers[vehicleID] == nil {
			m.reducers[vehicleID] = model.NewReducer()
		}
		reducer := m.reducers[vehicleID]

		// WebSocket connected successfully (no logging to avoid TUI disruption)

		// Start subscription in background
		go func() {
			// Subscribe to vehicle state
			subscription, err := rivian.SubscribeToVehicleState(m.ctx, wsClient, vehicleID)
			if err != nil {
				// Silently fail - user can manually refresh
				return
			}

			defer func() { _ = subscription.Close() }()

			for {
				select {
				case <-m.ctx.Done():
					_ = wsClient.Close()
					return

				case update := <-subscription.Updates():
					if update != nil {
						// Apply partial update through reducer
						event := model.PartialStateUpdate{
							VehicleID: vehicleID,
							Updates:   update,
						}
						finalState := reducer.Dispatch(event)

						// Save to store (if we have a complete state)
						if finalState != nil && m.store != nil {
							// Silently fail - not critical
							_ = m.store.SaveState(m.ctx, finalState)

							// Cache the state
							m.vehicleStates[vehicleID] = finalState

							// Send to update channel
							select {
							case updateChan <- finalState:
							default:
								// Channel full, skip update
							}
						}
					}
				}
			}
		}()

		// Return success message to trigger waitForUpdates
		return wsConnectedMsg{}
	}
}

func (m *Model) waitForUpdates() tea.Cmd {
	return func() tea.Msg {
		// Get the active vehicle's update channel
		if len(m.vehicles) == 0 {
			return nil
		}
		vehicleID := m.vehicles[m.activeVehicle].ID
		updateChan, ok := m.updateChans[vehicleID]
		if !ok {
			return nil
		}

		state := <-updateChan
		return stateUpdateMsg{state: state}
	}
}

// switchVehicle switches to a different vehicle
func (m *Model) switchVehicle(newIndex int) tea.Cmd {
	// Validate new index
	if newIndex < 0 || newIndex >= len(m.vehicles) {
		return nil
	}

	// Already on this vehicle
	if newIndex == m.activeVehicle {
		return nil
	}

	// Close old WebSocket connection
	oldVehicleID := m.vehicles[m.activeVehicle].ID
	if wsClient, ok := m.wsClients[oldVehicleID]; ok {
		_ = wsClient.Close()
	}

	// Switch to new vehicle
	m.activeVehicle = newIndex
	newVehicleID := m.vehicles[m.activeVehicle].ID

	// Update views with new vehicle ID
	m.healthView = NewHealthView(m.store, newVehicleID)
	m.chartsView = NewChartsView(m.store, newVehicleID)

	// Return commands to fetch state and subscribe
	return tea.Batch(
		m.fetchInitialState(),
		m.subscribeToUpdates(),
	)
}

// Rendering helpers

func (m *Model) renderHeader() string {
	if m.state == nil {
		return ""
	}

	// Vehicle info
	vehicleInfo := fmt.Sprintf("%s %s", m.state.Model, m.state.Name)
	if vehicleInfo == " " {
		vehicleInfo = "Rivian Vehicle"
	}

	// Online status
	status := "Online"
	statusColor := lipgloss.Color("#00ff00")
	if !m.state.IsOnline {
		status = "Offline"
		statusColor = lipgloss.Color("#ff0000")
	}

	// Last update
	updateTime := "never"
	if !m.lastUpdate.IsZero() {
		updateTime = m.lastUpdate.Format("15:04:05")
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#5f5fff")).
		Padding(0, 1)

	statusStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true)

	leftSection := headerStyle.Render(fmt.Sprintf("🚗 %s", vehicleInfo))
	rightSection := headerStyle.Render(fmt.Sprintf("%s | Updated: %s", statusStyle.Render(status), updateTime))

	// Calculate spacing
	spacingWidth := m.width - lipgloss.Width(leftSection) - lipgloss.Width(rightSection)
	if spacingWidth < 0 {
		spacingWidth = 0
	}
	spacing := headerStyle.Render(fmt.Sprintf("%*s", spacingWidth, ""))

	return leftSection + spacing + rightSection
}

func (m *Model) renderFooter() string {
	tabs := []string{
		"[1] Dashboard",
		"[2] Charge",
		"[3] Health",
		"[4] Charts",
	}

	activeTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ffff")).
		Bold(true)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	var renderedTabs []string
	for i, tab := range tabs {
		if ViewType(i) == m.currentView {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(tab))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(tab))
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Left, renderedTabs...)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	// Build help text with vehicle selector if multiple vehicles
	var helpText string
	if m.currentView == ViewCharts {
		// Charts view has special keyboard shortcuts
		if len(m.vehicles) > 1 {
			helpText = "[←/→] metric | [t] time | [v] vehicles | [r] refresh | [q] quit"
		} else {
			helpText = "[←/→] metric | [t] time | [r] refresh | [q] quit"
		}
	} else {
		if len(m.vehicles) > 1 {
			helpText = "[v] vehicles | [r] refresh | [q] quit"
		} else {
			helpText = "[r] refresh | [q] quit"
		}
	}
	help := helpStyle.Render(helpText)

	// Calculate spacing between tabs and help
	// Account for padding that will be added by the style (1 char on each side)
	tabWidth := lipgloss.Width(tabBar)
	helpWidth := lipgloss.Width(help)
	contentWidth := m.width - 2 // Reserve 2 chars for padding
	spacingWidth := contentWidth - tabWidth - helpWidth
	if spacingWidth < 0 {
		spacingWidth = 0
	}
	spacing := strings.Repeat(" ", spacingWidth)

	// Combine all parts to fill exactly contentWidth
	footerContent := tabBar + spacing + help

	// Apply background without setting explicit width (content already sized correctly)
	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1a1a1a")).
		Padding(0, 1)

	return footerStyle.Render(footerContent)
}

func (m *Model) renderLoading() string {
	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ffff")).
		Bold(true).
		Align(lipgloss.Center, lipgloss.Center).
		Width(m.width).
		Height(m.height)

	return loadingStyle.Render("Loading vehicle data...")
}

func (m *Model) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000")).
		Bold(true).
		Align(lipgloss.Center, lipgloss.Center).
		Width(m.width).
		Height(m.height)

	return errorStyle.Render(fmt.Sprintf("Error: %v\n\nPress 'r' to retry or 'q' to quit", m.err))
}
