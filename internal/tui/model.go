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
)

// Model is the main Bubble Tea model for the TUI
type Model struct {
	// Core dependencies
	client  rivian.Client
	store   *store.Store
	reducer *model.Reducer

	// Application state
	currentView ViewType
	state       *model.VehicleState
	err         error
	loading     bool

	// Subscription state
	ctx        context.Context
	cancel     context.CancelFunc
	wsClient   *rivian.WebSocketClient
	updateChan chan *model.VehicleState

	// Sub-views
	dashboardView *DashboardView
	chargeView    *ChargeView
	healthView    *HealthView

	// Terminal dimensions
	width  int
	height int

	// Last update time
	lastUpdate time.Time
}

// NewModel creates a new TUI model
func NewModel(client rivian.Client, store *store.Store, vehicleID string) *Model {
	ctx, cancel := context.WithCancel(context.Background())

	reducer := model.NewReducer()

	return &Model{
		client:        client,
		store:         store,
		reducer:       reducer,
		currentView:   ViewDashboard,
		loading:       true,
		ctx:           ctx,
		cancel:        cancel,
		updateChan:    make(chan *model.VehicleState, 10),
		dashboardView: NewDashboardView(),
		chargeView:    NewChargeView(),
		healthView:    NewHealthView(store, vehicleID),
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
	}

	// Render footer with keyboard shortcuts
	footer := m.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		footer,
	)
}

// handleKeyPress processes keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancel()
		return m, tea.Quit

	case "1", "d":
		m.currentView = ViewDashboard
		return m, nil

	case "2", "c":
		m.currentView = ViewCharge
		return m, nil

	case "3", "h":
		m.currentView = ViewHealth
		return m, nil

	case "r":
		// Refresh data
		return m, m.fetchInitialState()

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
		// Try to get latest state from API
		vehicles, err := m.client.GetVehicles(m.ctx)
		if err != nil {
			// Fall back to cached data - get latest state from last 30 days
			states, err := m.store.GetStateHistory(m.ctx, "", time.Now().Add(-30*24*time.Hour), 1)
			if err != nil || len(states) == 0 {
				return initialStateMsg{err: fmt.Errorf("failed to fetch vehicle data: %w", err)}
			}
			return initialStateMsg{state: states[0]}
		}

		if len(vehicles) == 0 {
			return initialStateMsg{err: fmt.Errorf("no vehicles found")}
		}

		// Get first vehicle
		vehicle := vehicles[0]

		// Dispatch VehicleListReceived to set identity
		event := model.VehicleListReceived{
			Vehicles: []rivian.Vehicle{vehicle},
			VehicleID: vehicle.ID,
		}
		m.reducer.Dispatch(event)

		// Get vehicle state
		rivState, err := m.client.GetVehicleState(m.ctx, vehicle.ID)
		if err != nil {
			return initialStateMsg{err: fmt.Errorf("failed to fetch vehicle state: %w", err)}
		}

		// Convert to domain model
		domainState := model.FromRivianVehicleState(rivState)

		// Dispatch VehicleStateReceived
		stateEvent := model.VehicleStateReceived{State: rivState}
		finalState := m.reducer.Dispatch(stateEvent)

		// Save to store
		if err := m.store.SaveState(m.ctx, domainState); err != nil {
			// Silently fail - not critical for TUI operation
		}

		return initialStateMsg{state: finalState}
	}
}

func (m *Model) subscribeToUpdates() tea.Cmd {
	return func() tea.Msg {
		vehicles, err := m.client.GetVehicles(m.ctx)
		if err != nil || len(vehicles) == 0 {
			// Non-fatal: silently continue without WebSocket
			return nil
		}

		vehicleID := vehicles[0].ID

		// Get HTTP client
		httpClient := m.client.(*rivian.HTTPClient)

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
		m.wsClient = wsClient

		// Connect
		if err := wsClient.Connect(m.ctx); err != nil {
			// Non-fatal: silently continue without WebSocket
			// The user can still manually refresh with 'r' key
			return nil
		}

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
					wsClient.Close()
					return

				case update := <-subscription.Updates():
					if update != nil {
						// Apply partial update through reducer
						event := model.PartialStateUpdate{
							VehicleID: vehicleID,
							Updates:   update,
						}
						finalState := m.reducer.Dispatch(event)

						// Save to store (if we have a complete state)
						if finalState != nil {
							if err := m.store.SaveState(m.ctx, finalState); err != nil {
								// Silently fail - not critical
							}

							// Send to update channel
							select {
							case m.updateChan <- finalState:
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
		state := <-m.updateChan
		return stateUpdateMsg{state: state}
	}
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

	help := helpStyle.Render("[r] refresh | [q] quit")

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
