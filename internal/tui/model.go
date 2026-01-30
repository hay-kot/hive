package tui

import (
	"context"
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
	isSplitMode bool        // true when width >= splitWidthThreshold
	focusedPane FocusedPane // which pane has focus (split mode)
	activeView  ViewType    // which view is shown (tab mode)

	// Messages
	msgStore     messaging.Store
	msgList      list.Model
	allMessages  []messaging.Message
	lastPollTime time.Time
	topicFilter  string

	// Message preview
	previewModal MessagePreviewModal
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
	l.Styles.Title = titleStyle
	l.Styles.TitleBar = lipgloss.NewStyle().PaddingLeft(1).PaddingBottom(1)
	l.FilterInput.PromptStyle = lipgloss.NewStyle().PaddingLeft(1).Foreground(colorBlue).Bold(true)
	l.FilterInput.Prompt = "Filter: "
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorBlue)

	handler := NewKeybindingHandler(cfg.Keybindings, service)

	// If no local remote detected, force show all
	showAll := opts.ShowAll || opts.LocalRemote == ""

	// Set initial title
	l.Title = buildTitle(showAll, opts.HideRecycled)

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

	// Create message list
	msgDelegate := NewMessageDelegate()
	ml := list.New([]list.Item{}, msgDelegate, 0, 0)
	ml.SetShowStatusBar(false)
	ml.SetFilteringEnabled(true)
	ml.Title = "Message Bus"
	ml.Styles.Title = titleStyle
	ml.Styles.TitleBar = lipgloss.NewStyle().PaddingLeft(1).PaddingBottom(1)
	ml.FilterInput.PromptStyle = lipgloss.NewStyle().PaddingLeft(1).Foreground(colorBlue).Bold(true)
	ml.FilterInput.Prompt = "Filter: "
	ml.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorBlue)

	// Add message list keybindings to help
	ml.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "preview"),
			),
			key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "switch view"),
			),
		}
	}

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
		msgList:      ml,
		topicFilter:  "*",
		focusedPane:  PaneSessions,
		activeView:   ViewSessions,
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
		m.isSplitMode = msg.Width >= splitWidthThreshold

		// Account for banner height (4 lines + margin)
		listHeight := msg.Height - 5
		if listHeight < 1 {
			listHeight = 1
		}

		if m.isSplitMode {
			// In split mode, we use custom focus indicators, so hide list titles
			// and account for the indicator row (-1)
			splitHeight := listHeight - 1
			if splitHeight < 1 {
				splitHeight = 1
			}
			paneWidth := (msg.Width - 1) / 2
			m.list.SetShowTitle(false)
			m.msgList.SetShowTitle(false)
			m.list.SetSize(paneWidth, splitHeight)
			m.msgList.SetSize(paneWidth, splitHeight)
		} else {
			// Full width for active view, show titles
			m.list.SetShowTitle(true)
			m.msgList.SetShowTitle(true)
			m.list.SetSize(msg.Width, listHeight)
			m.msgList.SetSize(msg.Width, listHeight)
		}
		return m, nil

	case messagesLoadedMsg:
		if msg.err != nil {
			// Silently ignore message loading errors
			return m, nil
		}
		// Append new messages if any
		if len(msg.messages) > 0 {
			m.allMessages = append(m.allMessages, msg.messages...)
			// Update message list items (newest first)
			items := make([]list.Item, len(m.allMessages))
			for i, message := range m.allMessages {
				// Reverse order so newest is at top
				items[len(m.allMessages)-1-i] = MessageItem{Message: message}
			}
			m.msgList.SetItems(items)
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

	// Update the focused list for any other messages
	var cmd tea.Cmd
	if m.isMessagesFocused() {
		m.msgList, cmd = m.msgList.Update(msg)
	} else {
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
	if m.list.SettingFilter() || m.msgList.SettingFilter() {
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
	case "enter":
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
	case "enter":
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
	switch keyStr {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "enter", "q":
		m.state = stateNormal
		return m, nil
	case "up", "k":
		m.previewModal.ScrollUp()
		return m, nil
	case "down", "j":
		m.previewModal.ScrollDown()
		return m, nil
	default:
		// Pass other messages to viewport for mouse wheel etc
		m.previewModal.UpdateViewport(msg)
		return m, nil
	}
}

// handleFilteringKey handles keys when filter input is active.
func (m Model) handleFilteringKey(msg tea.KeyMsg, keyStr string) (tea.Model, tea.Cmd) {
	if keyStr == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	if m.msgList.SettingFilter() {
		m.msgList, cmd = m.msgList.Update(msg)
	} else {
		m.list, cmd = m.list.Update(msg)
	}
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

	// Messages focused
	if keyStr == "enter" {
		// Open message preview modal
		selectedMsg := m.selectedMessage()
		if selectedMsg != nil {
			m.state = statePreviewingMessage
			m.previewModal = NewMessagePreviewModal(*selectedMsg, m.width, m.height)
			return m, nil
		}
	}

	// Pass other keys to msgList
	var cmd tea.Cmd
	m.msgList, cmd = m.msgList.Update(msg)
	return m, cmd
}

// handleTabKey handles tab key for switching focus/view.
func (m Model) handleTabKey() (tea.Model, tea.Cmd) {
	if m.isSplitMode {
		if m.focusedPane == PaneSessions {
			m.focusedPane = PaneMessages
		} else {
			m.focusedPane = PaneSessions
		}
	} else {
		if m.activeView == ViewSessions {
			m.activeView = ViewMessages
		} else {
			m.activeView = ViewSessions
		}
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
	item := m.msgList.SelectedItem()
	if item == nil {
		return nil
	}
	msgItem, ok := item.(MessageItem)
	if !ok {
		return nil
	}
	return &msgItem.Message
}

// isSessionsFocused returns true if the sessions pane is focused.
func (m Model) isSessionsFocused() bool {
	if m.isSplitMode {
		return m.focusedPane == PaneSessions
	}
	return m.activeView == ViewSessions
}

// isMessagesFocused returns true if the messages pane is focused.
func (m Model) isMessagesFocused() bool {
	if m.isSplitMode {
		return m.focusedPane == PaneMessages
	}
	return m.activeView == ViewMessages
}

// shouldPollMessages returns true if messages should be polled.
func (m Model) shouldPollMessages() bool {
	// In split mode, always poll
	if m.isSplitMode {
		return true
	}
	// In tab mode, only poll when messages view is active
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

	// Update title to show filter state
	m.list.Title = buildTitle(m.showAll, m.hideRecycled)

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

	// Build main view based on layout mode
	bannerView := bannerStyle.Render(banner)
	var contentView string
	if m.isSplitMode {
		contentView = m.renderSplitView()
	} else {
		contentView = m.renderTabView()
	}
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

// renderSplitView renders the two-column split layout.
func (m Model) renderSplitView() string {
	// Calculate pane widths (account for borders: 2 chars per pane + 1 gap)
	paneWidth := max((m.width-5)/2, 10)

	// Calculate content height (subtract border space)
	listHeight := max(m.height-4, 1) // Account for top/bottom borders

	// Render pane content
	leftContent := m.list.View()
	rightContent := m.msgList.View()

	// Build pane titles
	leftTitle := "Sessions"
	rightTitle := "Message Bus"

	// Apply focused/unfocused styles with borders
	var leftPane, rightPane string
	if m.focusedPane == PaneSessions {
		leftPane = focusedPaneStyle.
			Width(paneWidth).
			Height(listHeight).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(true).
			BorderRight(true).
			Render(viewSelectedStyle.Render("▸ "+leftTitle) + "\n" + leftContent)
		rightPane = unfocusedPaneStyle.
			Width(paneWidth).
			Height(listHeight).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(true).
			BorderRight(true).
			Render(viewNormalStyle.Render("  "+rightTitle) + "\n" + rightContent)
	} else {
		leftPane = unfocusedPaneStyle.
			Width(paneWidth).
			Height(listHeight).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(true).
			BorderRight(true).
			Render(viewNormalStyle.Render("  "+leftTitle) + "\n" + leftContent)
		rightPane = focusedPaneStyle.
			Width(paneWidth).
			Height(listHeight).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(true).
			BorderRight(true).
			Render(viewSelectedStyle.Render("▸ "+rightTitle) + "\n" + rightContent)
	}

	// Join panes horizontally with a small gap
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)
}

// renderTabView renders the single-view tab layout.
func (m Model) renderTabView() string {
	// Build view indicator
	var sessionsTab, messagesTab string
	if m.activeView == ViewSessions {
		sessionsTab = viewSelectedStyle.Render("[Sessions]")
		messagesTab = viewNormalStyle.Render(" Messages ")
	} else {
		sessionsTab = viewNormalStyle.Render(" Sessions ")
		messagesTab = viewSelectedStyle.Render("[Messages]")
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Left, "  ", sessionsTab, " | ", messagesTab)

	// Build content
	var content string
	if m.activeView == ViewSessions {
		content = m.list.View()
	} else {
		content = m.msgList.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content)
}

// buildTitle constructs the list title.
func buildTitle(showAll, hideRecycled bool) string {
	var scope string
	if showAll {
		scope = "all"
	} else {
		scope = "local"
	}

	if hideRecycled {
		return "Sessions (" + scope + ", active)"
	}
	return "Sessions (" + scope + ")"
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
