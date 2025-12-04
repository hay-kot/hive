package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/hive"
)

// UIState represents the current state of the TUI.
type UIState int

const (
	stateNormal UIState = iota
	stateConfirming
)

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	service  *hive.Service
	list     list.Model
	handler  *KeybindingHandler
	state    UIState
	modal    Modal
	pending  Action
	width    int
	height   int
	err      error
	quitting bool
}

// sessionsLoadedMsg is sent when sessions are loaded.
type sessionsLoadedMsg struct {
	sessions []session.Session
	err      error
}

// actionCompleteMsg is sent when an action completes.
type actionCompleteMsg struct {
	err error
}

// New creates a new TUI model.
func New(service *hive.Service, cfg *config.Config) Model {
	delegate := NewSessionDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Sessions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	handler := NewKeybindingHandler(cfg.Keybindings, service)

	return Model{
		service: service,
		list:    l,
		handler: handler,
		state:   stateNormal,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return m.loadSessions()
}

// loadSessions returns a command that loads sessions from the service.
func (m Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.service.ListSessions(context.Background())
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

// executeAction returns a command that executes the given action.
func (m Model) executeAction(action Action) tea.Cmd {
	return func() tea.Msg {
		err := m.handler.Execute(context.Background(), action)
		return actionCompleteMsg{err: err}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2) // Leave room for help bar
		return m, nil

	case sessionsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		items := make([]list.Item, len(msg.sessions))
		for i, s := range msg.sessions {
			items[i] = SessionItem{Session: s}
		}
		m.list.SetItems(items)
		return m, nil

	case actionCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		m.state = stateNormal
		m.pending = Action{}
		// Reload sessions after action
		return m, m.loadSessions()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// handleKey processes key presses.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle modal state
	if m.state == stateConfirming {
		switch key {
		case "y", "Y":
			m.state = stateNormal
			return m, m.executeAction(m.pending)
		case "n", "N", "esc":
			m.state = stateNormal
			m.pending = Action{}
			return m, nil
		}
		return m, nil
	}

	// Handle normal state
	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}

	// Check for configured keybindings
	selected := m.selectedSession()
	if selected == nil {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	action, ok := m.handler.Resolve(key, *selected)
	if ok {
		if action.NeedsConfirm() {
			m.state = stateConfirming
			m.pending = action
			m.modal = NewModal("Confirm", action.Confirm)
			return m, nil
		}
		return m, m.executeAction(action)
	}

	// Default list navigation
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// selectedSession returns the currently selected session, or nil if none.
func (m Model) selectedSession() *session.Session {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}
	sessionItem, ok := item.(SessionItem)
	if !ok {
		return nil
	}
	return &sessionItem.Session
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}

	// Build main view
	listView := m.list.View()
	helpBar := helpStyle.Render("[q] quit  " + m.handler.HelpString())
	mainView := lipgloss.JoinVertical(lipgloss.Left, listView, helpBar)

	// Overlay modal if confirming
	if m.state == stateConfirming {
		// Ensure we have dimensions
		w, h := m.width, m.height
		if w == 0 {
			w = 80
		}
		if h == 0 {
			h = 24
		}
		return m.modal.Overlay(mainView, w, h)
	}

	return mainView
}
