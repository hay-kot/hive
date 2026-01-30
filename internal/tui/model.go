package tui

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/messaging"
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
	stateRunningRecycle
	statePreviewingMessage
)

// Key constants for event handling.
const keyEnter = "enter"

// Options configures the TUI behavior.
type Options struct {
	ShowAll      bool            // Show all sessions vs only local repository
	LocalRemote  string          // Remote URL of current directory (empty if not in git repo)
	HideRecycled bool            // Hide recycled sessions by default
	MsgStore     messaging.Store // Message store for pub/sub events (optional)
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
	showAll      bool              // Toggle for showing all vs local sessions
	localRemote  string            // Remote URL of current directory
	hideRecycled bool              // Toggle for hiding recycled sessions
	allSessions  []session.Session // All sessions (unfiltered)

	// Recycle streaming state
	outputModal   OutputModal
	recycleOutput <-chan string
	recycleDone   <-chan error
	recycleCancel context.CancelFunc

	// Layout
	activeView ViewType // which view is shown

	// Messages
	msgStore     messaging.Store
	msgView      *MessagesView
	allMessages  []messaging.Message
	lastPollTime time.Time
	topicFilter  string

	// Message preview
	previewModal MessagePreviewModal

	// Clipboard
	copyCommand string
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

// recycleStartedMsg is sent when recycle begins with streaming output.
type recycleStartedMsg struct {
	output <-chan string
	done   <-chan error
	cancel context.CancelFunc
}

// recycleOutputMsg is sent when new output is available.
type recycleOutputMsg struct {
	line string
}

// recycleCompleteMsg is sent when recycle finishes.
type recycleCompleteMsg struct {
	err error
}

// New creates a new TUI model.
func New(service *hive.Service, cfg *config.Config, opts Options) Model {
	gitStatuses := kv.New[string, GitStatus]()

	delegate := NewSessionDelegate()
	delegate.GitStatuses = gitStatuses

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowTitle(false) // Title shown in tab bar instead
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.FilterInput.PromptStyle = lipgloss.NewStyle().PaddingLeft(1).Foreground(colorBlue).Bold(true)
	l.FilterInput.Prompt = "Filter: "
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorBlue)

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
		// Add toggle recycled keybinding
		bindings = append(bindings, key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "toggle recycled"),
		))
		// Add tab keybinding for view switching
		bindings = append(bindings, key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch view"),
		))
		return bindings
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	// Create message view
	msgView := NewMessagesView()

	return Model{
		service:      service,
		list:         l,
		handler:      handler,
		state:        stateNormal,
		spinner:      s,
		gitStatuses:  gitStatuses,
		gitWorkers:   cfg.Git.StatusWorkers,
		showAll:      showAll,
		localRemote:  opts.LocalRemote,
		hideRecycled: opts.HideRecycled,
		msgStore:     opts.MsgStore,
		msgView:      msgView,
		topicFilter:  "*",
		activeView:   ViewSessions,
		copyCommand:  cfg.Commands.CopyCommand,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.loadSessions(), m.spinner.Tick}
	// Start message polling if we have a store
	if m.msgStore != nil {
		cmds = append(cmds, loadMessages(m.msgStore, m.topicFilter, time.Time{}))
		cmds = append(cmds, schedulePollTick())
	}
	return tea.Batch(cmds...)
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

		// Account for: banner (4 lines + 1 margin) + tab bar (1) + subheading (1) = 7 lines
		contentHeight := msg.Height - 7
		if contentHeight < 1 {
			contentHeight = 1
		}

		m.list.SetSize(msg.Width, contentHeight)
		m.msgView.SetSize(msg.Width, contentHeight)
		return m, nil

	case messagesLoadedMsg:
		if msg.err != nil {
			// Silently ignore message loading errors
			return m, nil
		}
		// Append new messages if any
		if len(msg.messages) > 0 {
			m.allMessages = append(m.allMessages, msg.messages...)
			// Update message view with reversed order (newest first)
			reversed := make([]messaging.Message, len(m.allMessages))
			for i, message := range m.allMessages {
				reversed[len(m.allMessages)-1-i] = message
			}
			m.msgView.SetMessages(reversed)
		}
		// Always update poll time so we don't re-fetch the same messages
		m.lastPollTime = time.Now()
		return m, nil

	case pollTickMsg:
		// Only poll if messages are visible
		if m.shouldPollMessages() && m.msgStore != nil {
			return m, tea.Batch(
				loadMessages(m.msgStore, m.topicFilter, m.lastPollTime),
				schedulePollTick(),
			)
		}
		// Keep scheduling poll ticks even if not actively polling
		return m, schedulePollTick()

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

	case recycleStartedMsg:
		m.state = stateRunningRecycle
		m.outputModal = NewOutputModal("Recycling session...")
		m.recycleOutput = msg.output
		m.recycleDone = msg.done
		m.recycleCancel = msg.cancel
		return m, tea.Batch(
			listenForRecycleOutput(msg.output, msg.done),
			m.outputModal.Spinner().Tick,
		)

	case recycleOutputMsg:
		m.outputModal.AddLine(msg.line)
		// Keep listening for more output
		return m, listenForRecycleOutput(m.recycleOutput, m.recycleDone)

	case recycleCompleteMsg:
		m.outputModal.SetComplete(msg.err)
		m.recycleOutput = nil
		m.recycleDone = nil
		m.recycleCancel = nil
		// Stay in stateRunningRecycle until user dismisses
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Update the focused list for any other messages (only session list needs this)
	var cmd tea.Cmd
	if !m.isMessagesFocused() {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

// handleKey processes key presses.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyStr := msg.String()

	// Handle modal states first
	if m.state == statePreviewingMessage {
		return m.handlePreviewModalKey(msg, keyStr)
	}
	if m.state == stateRunningRecycle {
		return m.handleRecycleModalKey(keyStr)
	}
	if m.state == stateConfirming {
		return m.handleConfirmModalKey(keyStr)
	}

	// When filtering in either list, pass most keys except quit
	if m.list.SettingFilter() || m.msgView.IsFiltering() {
		return m.handleFilteringKey(msg, keyStr)
	}

	// Handle normal state
	return m.handleNormalKey(msg, keyStr)
}

// handleRecycleModalKey handles keys when recycle modal is shown.
func (m Model) handleRecycleModalKey(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "ctrl+c":
		if m.recycleCancel != nil {
			m.recycleCancel()
		}
		m.quitting = true
		return m, tea.Quit
	case "esc":
		if m.outputModal.IsRunning() && m.recycleCancel != nil {
			m.recycleCancel()
		}
		m.state = stateNormal
		m.pending = Action{}
		return m, m.loadSessions()
	case keyEnter:
		if !m.outputModal.IsRunning() {
			m.state = stateNormal
			m.pending = Action{}
			return m, m.loadSessions()
		}
	}
	return m, nil
}

// handleConfirmModalKey handles keys when confirmation modal is shown.
func (m Model) handleConfirmModalKey(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case keyEnter:
		m.state = stateNormal
		if m.modal.ConfirmSelected() {
			action := m.pending
			if action.Type == ActionTypeRecycle {
				return m, m.startRecycle(action.SessionID)
			}
			return m, m.executeAction(action)
		}
		m.pending = Action{}
		return m, nil
	case "esc":
		m.state = stateNormal
		m.pending = Action{}
		return m, nil
	case "left", "right", "h", "l", "tab":
		m.modal.ToggleSelection()
		return m, nil
	}
	return m, nil
}

// handlePreviewModalKey handles keys when message preview modal is shown.
func (m Model) handlePreviewModalKey(msg tea.KeyMsg, keyStr string) (tea.Model, tea.Cmd) {
	// Clear copy status on any key press
	m.previewModal.ClearCopyStatus()

	switch keyStr {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", keyEnter, "q":
		m.state = stateNormal
		return m, nil
	case "up", "k":
		m.previewModal.ScrollUp()
		return m, nil
	case "down", "j":
		m.previewModal.ScrollDown()
		return m, nil
	case "c", "y":
		// Copy payload to clipboard
		if err := m.copyToClipboard(m.previewModal.Payload()); err != nil {
			m.previewModal.SetCopyStatus("Copy failed: " + err.Error())
		} else {
			m.previewModal.SetCopyStatus("Copied!")
		}
		return m, nil
	default:
		// Pass other messages to viewport for mouse wheel etc
		m.previewModal.UpdateViewport(msg)
		return m, nil
	}
}

// copyToClipboard copies the given text to the system clipboard.
func (m Model) copyToClipboard(text string) error {
	if m.copyCommand == "" {
		return nil
	}

	// Split the command into program and args
	parts := strings.Fields(m.copyCommand)
	if len(parts) == 0 {
		return nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// handleFilteringKey handles keys when filter input is active.
func (m Model) handleFilteringKey(msg tea.KeyMsg, keyStr string) (tea.Model, tea.Cmd) {
	if keyStr == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	// Handle message view filtering
	if m.msgView.IsFiltering() {
		switch keyStr {
		case "esc":
			m.msgView.CancelFilter()
		case keyEnter:
			m.msgView.ConfirmFilter()
		case "backspace":
			m.msgView.DeleteFilterRune()
		default:
			// Add character to filter if it's a printable rune
			if len(msg.Runes) > 0 {
				for _, r := range msg.Runes {
					m.msgView.AddFilterRune(r)
				}
			}
		}
		return m, nil
	}

	// Handle session list filtering
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// handleNormalKey handles keys in normal state.
func (m Model) handleNormalKey(msg tea.KeyMsg, keyStr string) (tea.Model, tea.Cmd) {
	// Global keys that work regardless of focus
	switch keyStr {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "tab":
		return m.handleTabKey()
	}

	// Session-specific keys only when sessions focused
	if m.isSessionsFocused() {
		switch keyStr {
		case "g":
			return m, m.refreshGitStatuses()
		case "a":
			if m.localRemote != "" {
				m.showAll = !m.showAll
				return m.applyFilter()
			}
		case "x":
			m.hideRecycled = !m.hideRecycled
			return m.applyFilter()
		}
		return m.handleSessionsKey(msg, keyStr)
	}

	// Messages view focused - handle navigation
	switch keyStr {
	case keyEnter:
		// Open message preview modal
		selectedMsg := m.selectedMessage()
		if selectedMsg != nil {
			m.state = statePreviewingMessage
			m.previewModal = NewMessagePreviewModal(*selectedMsg, m.width, m.height)
		}
	case "up", "k":
		m.msgView.MoveUp()
	case "down", "j":
		m.msgView.MoveDown()
	case "/":
		m.msgView.StartFilter()
	}
	return m, nil
}

// handleTabKey handles tab key for switching views.
func (m Model) handleTabKey() (tea.Model, tea.Cmd) {
	if m.activeView == ViewSessions {
		m.activeView = ViewMessages
	} else {
		m.activeView = ViewSessions
	}
	return m, nil
}

// handleSessionsKey handles keys when sessions pane is focused.
func (m Model) handleSessionsKey(msg tea.KeyMsg, keyStr string) (tea.Model, tea.Cmd) {
	selected := m.selectedSession()
	if selected == nil {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	action, ok := m.handler.Resolve(keyStr, *selected)
	if ok {
		if action.NeedsConfirm() {
			m.state = stateConfirming
			m.pending = action
			m.modal = NewModal("Confirm", action.Confirm)
			return m, nil
		}
		if action.Type == ActionTypeRecycle {
			return m, m.startRecycle(action.SessionID)
		}
		m.state = stateLoading
		m.loadingMessage = "Processing..."
		return m, m.executeAction(action)
	}

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

// selectedMessage returns the currently selected message, or nil if none.
func (m Model) selectedMessage() *messaging.Message {
	return m.msgView.SelectedMessage()
}

// isSessionsFocused returns true if the sessions view is active.
func (m Model) isSessionsFocused() bool {
	return m.activeView == ViewSessions
}

// isMessagesFocused returns true if the messages view is active.
func (m Model) isMessagesFocused() bool {
	return m.activeView == ViewMessages
}

// shouldPollMessages returns true if messages should be polled.
func (m Model) shouldPollMessages() bool {
	return m.activeView == ViewMessages
}

// applyFilter filters sessions based on showAll and hideRecycled flags.
func (m Model) applyFilter() (tea.Model, tea.Cmd) {
	var filtered []session.Session
	for _, s := range m.allSessions {
		// Filter by local remote
		if !m.showAll && m.localRemote != "" && s.Remote != m.localRemote {
			continue
		}
		// Filter recycled sessions
		if m.hideRecycled && s.State == session.StateRecycled {
			continue
		}
		filtered = append(filtered, s)
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

	// Build main view
	bannerView := bannerStyle.Render(banner)
	contentView := m.renderTabView()
	mainView := lipgloss.JoinVertical(lipgloss.Left, bannerView, contentView)

	// Ensure we have dimensions for modals
	w, h := m.width, m.height
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	// Overlay output modal if running recycle
	if m.state == stateRunningRecycle {
		return m.outputModal.Overlay(mainView, w, h)
	}

	// Overlay message preview modal
	if m.state == statePreviewingMessage {
		return m.previewModal.Overlay(mainView, w, h)
	}

	// Overlay loading spinner if loading
	if m.state == stateLoading {
		loadingView := lipgloss.JoinHorizontal(lipgloss.Left, m.spinner.View(), " "+m.loadingMessage)
		modal := NewModal("", loadingView)
		return modal.Overlay(mainView, w, h)
	}

	// Overlay modal if confirming
	if m.state == stateConfirming {
		return m.modal.Overlay(mainView, w, h)
	}

	return mainView
}

// renderTabView renders the tab-based view layout.
func (m Model) renderTabView() string {
	// Build tab bar
	var sessionsTab, messagesTab string
	if m.activeView == ViewSessions {
		sessionsTab = viewSelectedStyle.Render("Sessions")
		messagesTab = viewNormalStyle.Render("Messages")
	} else {
		sessionsTab = viewNormalStyle.Render("Sessions")
		messagesTab = viewSelectedStyle.Render("Messages")
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Left, " ", sessionsTab, " | ", messagesTab)

	// Build subheading (always present to prevent layout shift)
	subheading := m.buildSubheading()

	// Build content
	var content string
	if m.activeView == ViewSessions {
		content = m.list.View()
	} else {
		content = m.msgView.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, subheading, content)
}

// buildSubheading constructs the subheading for the current view.
func (m Model) buildSubheading() string {
	if m.activeView == ViewSessions {
		var indicators []string
		if !m.showAll && m.localRemote != "" {
			indicators = append(indicators, "local")
		}
		if m.hideRecycled {
			indicators = append(indicators, "active")
		}
		if len(indicators) > 0 {
			subStyle := lipgloss.NewStyle().Foreground(colorGray)
			return " " + subStyle.Render("showing "+strings.Join(indicators, ", "))
		}
	}
	// Empty line to maintain consistent layout
	return ""
}

// startRecycle returns a command that starts the recycle operation with streaming output.
func (m Model) startRecycle(sessionID string) tea.Cmd {
	return func() tea.Msg {
		output := make(chan string, 100)
		done := make(chan error, 1)

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			defer close(output)
			defer close(done)

			writer := &channelWriter{ch: output, ctx: ctx}
			err := m.service.RecycleSession(ctx, sessionID, writer)
			done <- err
		}()

		return recycleStartedMsg{
			output: output,
			done:   done,
			cancel: cancel,
		}
	}
}

// listenForRecycleOutput returns a command that waits for the next output or completion.
func listenForRecycleOutput(output <-chan string, done <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case line, ok := <-output:
			if !ok {
				// Output channel closed, wait for done
				err := <-done
				return recycleCompleteMsg{err: err}
			}
			return recycleOutputMsg{line: line}
		case err := <-done:
			return recycleCompleteMsg{err: err}
		}
	}
}

// channelWriter is an io.Writer that sends writes to a channel.
// It respects context cancellation to avoid blocking or panicking.
type channelWriter struct {
	ch  chan<- string
	ctx context.Context
}

func (w *channelWriter) Write(p []byte) (int, error) {
	select {
	case w.ch <- string(p):
		return len(p), nil
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	}
}
