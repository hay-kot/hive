package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/pkg/kv"
)

// UIState represents the current state of the TUI.
type UIState int

const (
	stateNormal UIState = iota
	stateConfirming
	stateLoading
)

// Options configures the TUI behavior.
type Options struct {
	ShowAll     bool   // Show all sessions vs only local repository
	LocalRemote string // Remote URL of current directory (empty if not in git repo)
}

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	service        *hive.Service
	list           list.Model
	handler        *KeybindingHandler
	state          UIState
	modal          Modal
	pending        Action
	width          int
	height         int
	err            error
	spinner        spinner.Model
	loadingMessage string
	quitting       bool
	gitStatuses    *kv.Store[string, GitStatus]
	gitWorkers     int

	// Filtering
	showAll     bool              // Toggle for showing all vs local sessions
	localRemote string            // Remote URL of current directory
	allSessions []session.Session // All sessions (unfiltered)
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
func New(service *hive.Service, cfg *config.Config, opts Options) Model {
	gitStatuses := kv.New[string, GitStatus]()

	delegate := NewSessionDelegate()
	delegate.GitStatuses = gitStatuses

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Sessions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.TitleBar = lipgloss.NewStyle().PaddingLeft(1)

	handler := NewKeybindingHandler(cfg.Keybindings, service)

	// If no local remote detected, force show all
	showAll := opts.ShowAll || opts.LocalRemote == ""

	// Add custom keybindings to list help
	l.AdditionalShortHelpKeys = func() []key.Binding {
		bindings := handler.KeyBindings()
		// Add git refresh keybinding
		bindings = append(bindings, key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "refresh git"),
		))
		// Add toggle all keybinding
		bindings = append(bindings, key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "toggle all"),
		))
		return bindings
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return Model{
		service:     service,
		list:        l,
		handler:     handler,
		state:       stateNormal,
		spinner:     s,
		gitStatuses: gitStatuses,
		gitWorkers:  cfg.Git.StatusWorkers,
		showAll:     showAll,
		localRemote: opts.LocalRemote,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadSessions(), m.spinner.Tick)
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
		// Account for banner height (4 lines + margin)
		listHeight := msg.Height - 5
		if listHeight < 1 {
			listHeight = 1
		}
		m.list.SetSize(msg.Width, listHeight)
		return m, nil

	case sessionsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateNormal
			return m, nil
		}
		// Store all sessions for filtering
		m.allSessions = msg.sessions
		// Apply filter and update list
		return m.applyFilter()

	case gitStatusBatchCompleteMsg:
		m.gitStatuses.SetBatch(msg.Results)
		return m, nil

	case actionCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateNormal
			m.pending = Action{}
			return m, nil
		}
		m.state = stateNormal
		m.pending = Action{}
		// Reload sessions after action
		return m, m.loadSessions()

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
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
	case "g":
		// Refresh git status for all sessions
		return m, m.refreshGitStatuses()
	case "a":
		// Toggle between local and all sessions
		if m.localRemote != "" {
			m.showAll = !m.showAll
			return m.applyFilter()
		}
		return m, nil
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
		// Show loading state for action
		m.state = stateLoading
		m.loadingMessage = "Processing..."
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

// applyFilter filters sessions based on showAll flag and updates the list.
func (m Model) applyFilter() (tea.Model, tea.Cmd) {
	var filtered []session.Session
	if m.showAll || m.localRemote == "" {
		filtered = m.allSessions
	} else {
		for _, s := range m.allSessions {
			if s.Remote == m.localRemote {
				filtered = append(filtered, s)
			}
		}
	}

	items := make([]list.Item, len(filtered))
	paths := make([]string, len(filtered))
	for i, s := range filtered {
		items[i] = SessionItem{Session: s}
		paths[i] = s.Path
		m.gitStatuses.Set(s.Path, GitStatus{IsLoading: true})
	}
	m.list.SetItems(items)
	m.state = stateNormal

	// Update title to show filter state
	if m.showAll || m.localRemote == "" {
		m.list.Title = "Sessions (all)"
	} else {
		m.list.Title = "Sessions (local)"
	}

	if len(paths) == 0 {
		return m, nil
	}
	return m, fetchGitStatusBatch(m.service.Git(), paths, m.gitWorkers)
}

// refreshGitStatuses returns a command that refreshes git status for all sessions.
func (m Model) refreshGitStatuses() tea.Cmd {
	items := m.list.Items()
	paths := make([]string, 0, len(items))

	for _, item := range items {
		sessionItem, ok := item.(SessionItem)
		if !ok {
			continue
		}
		path := sessionItem.Session.Path
		paths = append(paths, path)
		// Mark as loading
		m.gitStatuses.Set(path, GitStatus{IsLoading: true})
	}

	if len(paths) == 0 {
		return nil
	}

	return fetchGitStatusBatch(m.service.Git(), paths, m.gitWorkers)
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// Build main view with banner
	bannerView := bannerStyle.Render(banner)
	mainView := lipgloss.JoinVertical(lipgloss.Left, bannerView, m.list.View())

	// Overlay loading spinner if loading
	if m.state == stateLoading {
		w, h := m.width, m.height
		if w == 0 {
			w = 80
		}
		if h == 0 {
			h = 24
		}
		loadingView := lipgloss.JoinHorizontal(lipgloss.Left, m.spinner.View(), " "+m.loadingMessage)
		modal := NewModal("", loadingView)
		return modal.Overlay(mainView, w, h)
	}

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
