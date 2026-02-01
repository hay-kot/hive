package tui

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/huh"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/messaging"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/internal/integration/terminal"
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
	stateCreatingSession
)

// Key constants for event handling.
const (
	keyEnter = "enter"
	keyCtrlC = "ctrl+c"
)

// Options configures the TUI behavior.
type Options struct {
	LocalRemote     string            // Remote URL of current directory (empty if not in git repo)
	MsgStore        messaging.Store   // Message store for pub/sub events (optional)
	TerminalManager *terminal.Manager // Terminal integration manager (optional)
}

// PendingCreate holds data for a session to create after TUI exits.
type PendingCreate struct {
	Remote string
	Name   string
}

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	cfg            *config.Config
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
	columnWidths   *ColumnWidths

	// Terminal integration
	terminalManager  *terminal.Manager
	terminalStatuses *kv.Store[string, TerminalStatus]

	// Status animation
	animationFrame int
	treeDelegate   TreeDelegate // Keep reference to update animation frame

	// Filtering
	localRemote string            // Remote URL of current directory (for highlighting)
	allSessions []session.Session // All sessions (unfiltered)

	// Recycle streaming state
	outputModal   OutputModal
	recycleOutput <-chan string
	recycleDone   <-chan error
	recycleCancel context.CancelFunc

	// Layout
	activeView ViewType // which view is shown
	refreshing bool     // true during background session refresh

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

	// New session form
	repoDirs        []string
	discoveredRepos []DiscoveredRepo
	newSessionForm  *NewSessionForm

	// Pending action for after TUI exits
	pendingCreate *PendingCreate
}

// PendingCreate returns any pending session creation data.
func (m Model) PendingCreate() *PendingCreate {
	return m.pendingCreate
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

// reposDiscoveredMsg is sent when repository scanning completes.
type reposDiscoveredMsg struct {
	repos []DiscoveredRepo
}

// New creates a new TUI model.
func New(service *hive.Service, cfg *config.Config, opts Options) Model {
	gitStatuses := kv.New[string, GitStatus]()
	terminalStatuses := kv.New[string, TerminalStatus]()
	columnWidths := &ColumnWidths{}

	delegate := NewTreeDelegate()
	delegate.GitStatuses = gitStatuses
	delegate.TerminalStatuses = terminalStatuses
	delegate.ColumnWidths = columnWidths

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowTitle(false) // Title shown in tab bar instead
	l.Styles.TitleBar = lipgloss.NewStyle()
	// Configure filter input styles for bubbles v2
	l.FilterInput.Prompt = "Filter: "
	filterStyles := textinput.DefaultStyles(true) // dark mode
	filterStyles.Focused.Prompt = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("#7aa2f7")).Bold(true)
	filterStyles.Cursor.Color = lipgloss.Color("#7aa2f7")
	l.FilterInput.SetStyles(filterStyles)

	// Style help to match messages view (consistent gray, bullet separators, left padding)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	l.Help.Styles.ShortKey = helpStyle
	l.Help.Styles.ShortDesc = helpStyle
	l.Help.Styles.ShortSeparator = helpStyle
	l.Help.Styles.FullKey = helpStyle
	l.Help.Styles.FullDesc = helpStyle
	l.Help.Styles.FullSeparator = helpStyle
	l.Help.ShortSeparator = " • "
	l.Styles.HelpStyle = lipgloss.NewStyle().PaddingLeft(1)

	handler := NewKeybindingHandler(cfg.Keybindings, service)

	// Add custom keybindings to list help
	l.AdditionalShortHelpKeys = func() []key.Binding {
		bindings := handler.KeyBindings()
		// Add git refresh keybinding
		bindings = append(bindings, key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "refresh git"),
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
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")) // blue, lipgloss v1 for bubbles v1

	// Create message view
	msgView := NewMessagesView()

	return Model{
		cfg:              cfg,
		service:          service,
		list:             l,
		handler:          handler,
		state:            stateNormal,
		spinner:          s,
		gitStatuses:      gitStatuses,
		gitWorkers:       cfg.Git.StatusWorkers,
		columnWidths:     columnWidths,
		terminalManager:  opts.TerminalManager,
		terminalStatuses: terminalStatuses,
		treeDelegate:     delegate,
		localRemote:      opts.LocalRemote,
		msgStore:         opts.MsgStore,
		msgView:          msgView,
		topicFilter:      "*",
		activeView:       ViewSessions,
		copyCommand:      cfg.Commands.CopyCommand,
		repoDirs:         cfg.RepoDirs,
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
	// Start session refresh timer
	if cmd := m.scheduleSessionRefresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// Scan for repositories if configured
	if len(m.repoDirs) > 0 {
		cmds = append(cmds, m.scanRepoDirs())
	}
	// Start terminal status polling and animation if integration is enabled
	if m.terminalManager != nil && m.terminalManager.HasEnabledIntegrations() {
		cmds = append(cmds, startTerminalPollTicker(m.cfg.Integrations.Terminal.PollInterval))
		cmds = append(cmds, scheduleAnimationTick())
	}
	return tea.Batch(cmds...)
}

// scanRepoDirs returns a command that scans configured directories for git repositories.
func (m Model) scanRepoDirs() tea.Cmd {
	return func() tea.Msg {
		repos, _ := ScanRepoDirs(context.Background(), m.repoDirs, m.service.Git())
		return reposDiscoveredMsg{repos: repos}
	}
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

		// Account for: banner (5 lines) + tab bar (1) = 6 lines
		// (spacing line is included in list's titleView and msgView's prefix)
		contentHeight := msg.Height - 6
		if contentHeight < 1 {
			contentHeight = 1
		}

		m.list.SetSize(msg.Width, contentHeight)
		// msgView gets -1 because we prepend a blank line for consistent spacing
		m.msgView.SetSize(msg.Width, contentHeight-1)
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

	case sessionRefreshTickMsg:
		// Refresh sessions when Sessions view is active and no modal open
		if m.activeView == ViewSessions && !m.isModalActive() {
			m.refreshing = true
			return m, tea.Batch(
				m.loadSessions(),
				m.scheduleSessionRefresh(),
			)
		}
		// Keep scheduling refresh ticks even if not actively refreshing
		return m, m.scheduleSessionRefresh()

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
		m.refreshing = false
		return m, nil

	case terminalPollTickMsg:
		// Start next poll cycle
		var cmds []tea.Cmd
		sessions := make([]*session.Session, len(m.allSessions))
		for i := range m.allSessions {
			sessions[i] = &m.allSessions[i]
		}
		cmds = append(cmds, fetchTerminalStatusBatch(m.terminalManager, sessions, m.gitWorkers))
		if m.terminalManager != nil && m.terminalManager.HasEnabledIntegrations() {
			cmds = append(cmds, startTerminalPollTicker(m.cfg.Integrations.Terminal.PollInterval))
		}
		return m, tea.Batch(cmds...)

	case terminalStatusBatchCompleteMsg:
		if m.terminalStatuses != nil {
			m.terminalStatuses.SetBatch(msg.Results)
		}
		return m, nil

	case animationTickMsg:
		// Advance animation frame
		m.animationFrame = (m.animationFrame + 1) % AnimationFrameCount
		// Update the delegate with new frame
		m.treeDelegate.AnimationFrame = m.animationFrame
		m.list.SetDelegate(m.treeDelegate)
		// Schedule next tick
		return m, scheduleAnimationTick()

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

	case reposDiscoveredMsg:
		m.discoveredRepos = msg.repos
		// Update help to include 'n' keybinding if repos were discovered
		if len(msg.repos) > 0 {
			m.list.AdditionalShortHelpKeys = func() []key.Binding {
				bindings := m.handler.KeyBindings()
				bindings = append(bindings, key.NewBinding(
					key.WithKeys("n"),
					key.WithHelp("n", "new session"),
				))
				bindings = append(bindings, key.NewBinding(
					key.WithKeys("g"),
					key.WithHelp("g", "refresh git"),
				))
				bindings = append(bindings, key.NewBinding(
					key.WithKeys("tab"),
					key.WithHelp("tab", "switch view"),
				))
				return bindings
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Route all other messages to the form when creating session
	if m.state == stateCreatingSession && m.newSessionForm != nil {
		return m.updateNewSessionForm(msg)
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
	if m.state == stateCreatingSession {
		return m.handleNewSessionFormKey(msg, keyStr)
	}
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

// handleNewSessionFormKey handles keys when new session form is shown.
func (m Model) handleNewSessionFormKey(msg tea.KeyMsg, keyStr string) (tea.Model, tea.Cmd) {
	if keyStr == keyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	// Handle esc to close dialog
	if keyStr == "esc" {
		m.newSessionForm.SetCancelled()
		m.state = stateNormal
		m.newSessionForm = nil
		return m, nil
	}

	// Pass to form
	return m.updateNewSessionForm(msg)
}

// updateNewSessionForm routes any message to the form and handles state changes.
func (m Model) updateNewSessionForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Note: huh uses bubbletea v1, so we ignore its commands (incompatible with v2)
	// The form still works for input, just without v1-specific command handling
	form, _ := m.newSessionForm.Form().Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.newSessionForm.form = f

		// Check if form completed - set pending create and exit TUI
		if f.State == huh.StateCompleted {
			result := m.newSessionForm.Result()
			m.state = stateNormal
			m.newSessionForm = nil
			m.pendingCreate = &PendingCreate{
				Remote: result.Repo.Remote,
				Name:   result.SessionName,
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

// handleRecycleModalKey handles keys when recycle modal is shown.
func (m Model) handleRecycleModalKey(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case keyCtrlC:
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
	case keyCtrlC:
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
	if keyStr == keyCtrlC {
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
			// In bubbletea V2, msg.Runes is replaced with msg.Key().Text
			if text := msg.Key().Text; text != "" {
				for _, r := range text {
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
	case "q", keyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case "tab":
		return m.handleTabKey()
	}

	// Session-specific keys only when sessions focused
	if m.isSessionsFocused() {
		if keyStr == "g" {
			return m, m.refreshGitStatuses()
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
	// Handle 'n' for new session (only if repos are discovered)
	if keyStr == "n" && len(m.discoveredRepos) > 0 {
		// Determine preselected remote
		preselectedRemote := m.localRemote
		if selected := m.selectedSession(); selected != nil {
			preselectedRemote = selected.Remote
		}
		// Build map of existing session names for validation
		existingNames := make(map[string]bool, len(m.allSessions))
		for _, s := range m.allSessions {
			existingNames[s.Name] = true
		}
		m.newSessionForm = NewNewSessionForm(m.discoveredRepos, preselectedRemote, existingNames)
		m.state = stateCreatingSession
		// Note: huh uses bubbletea v1, so we can't use its Init() command directly
		// The form will still work, just without v1-specific initialization
		_ = m.newSessionForm.Form().Init()
		return m, nil
	}

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
		// If exit is requested, execute synchronously and quit immediately
		// This avoids async message flow issues in some terminal contexts (e.g., tmux popups)
		if action.Exit {
			_ = m.handler.Execute(context.Background(), action)
			m.quitting = true
			return m, tea.Quit
		}
		// Store pending action for exit check after completion
		m.pending = action
		if !action.Silent {
			m.state = stateLoading
			m.loadingMessage = "Processing..."
		}
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
	// Handle TreeItem (tree view mode)
	if treeItem, ok := item.(TreeItem); ok {
		if treeItem.IsHeader {
			return nil // Headers aren't sessions
		}
		return &treeItem.Session
	}
	return nil
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

// isModalActive returns true if any modal is currently open.
func (m Model) isModalActive() bool {
	return m.state != stateNormal
}

// applyFilter rebuilds the tree view from all sessions.
func (m Model) applyFilter() (tea.Model, tea.Cmd) {
	// Group sessions by repository and build tree items
	groups := GroupSessionsByRepo(m.allSessions, m.localRemote)
	items := BuildTreeItems(groups, m.localRemote)

	// Calculate column widths across all sessions
	*m.columnWidths = CalculateColumnWidths(m.allSessions, nil)

	// Collect paths for git status fetching
	// During background refresh, keep existing statuses to avoid flashing
	paths := make([]string, 0, len(m.allSessions))
	for _, s := range m.allSessions {
		paths = append(paths, s.Path)
		if !m.refreshing {
			m.gitStatuses.Set(s.Path, GitStatus{IsLoading: true})
		}
	}

	m.list.SetItems(items)
	m.state = stateNormal

	if len(paths) == 0 {
		m.refreshing = false
		return m, nil
	}
	// refreshing is cleared when gitStatusBatchCompleteMsg is received
	return m, fetchGitStatusBatch(m.service.Git(), paths, m.gitWorkers)
}

// refreshGitStatuses returns a command that refreshes git status for all sessions.
func (m Model) refreshGitStatuses() tea.Cmd {
	items := m.list.Items()
	paths := make([]string, 0, len(items))

	for _, item := range items {
		treeItem, ok := item.(TreeItem)
		if !ok || treeItem.IsHeader {
			continue
		}
		path := treeItem.Session.Path
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
func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
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

	// Helper to create view with alt screen enabled
	newView := func(content string) tea.View {
		v := tea.NewView(content)
		v.AltScreen = true
		return v
	}

	// Overlay output modal if running recycle
	if m.state == stateRunningRecycle {
		return newView(m.outputModal.Overlay(mainView, w, h))
	}

	// Overlay new session form (render directly without Modal's Confirm/Cancel buttons)
	if m.state == stateCreatingSession && m.newSessionForm != nil {
		formContent := lipgloss.JoinVertical(
			lipgloss.Left,
			modalTitleStyle.Render("New Session"),
			"",
			m.newSessionForm.View(),
		)
		formOverlay := modalStyle.Render(formContent)

		// Use Compositor/Layer for true overlay (background remains visible)
		bgLayer := lipgloss.NewLayer(mainView)
		formLayer := lipgloss.NewLayer(formOverlay)
		formW := lipgloss.Width(formOverlay)
		formH := lipgloss.Height(formOverlay)
		centerX := (w - formW) / 2
		centerY := (h - formH) / 2
		formLayer.X(centerX).Y(centerY).Z(1)

		compositor := lipgloss.NewCompositor(bgLayer, formLayer)
		return newView(compositor.Render())
	}

	// Overlay message preview modal
	if m.state == statePreviewingMessage {
		return newView(m.previewModal.Overlay(mainView, w, h))
	}

	// Overlay loading spinner if loading
	if m.state == stateLoading {
		loadingView := lipgloss.JoinHorizontal(lipgloss.Left, m.spinner.View(), " "+m.loadingMessage)
		modal := NewModal("", loadingView)
		return newView(modal.Overlay(mainView, w, h))
	}

	// Overlay modal if confirming
	if m.state == stateConfirming {
		return newView(m.modal.Overlay(mainView, w, h))
	}

	return newView(mainView)
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
	tabBarContent := lipgloss.JoinHorizontal(lipgloss.Left, sessionsTab, " | ", messagesTab)
	tabBar := lipgloss.NewStyle().PaddingLeft(1).Render(tabBarContent)

	// Calculate content height: total - banner (5) - tab bar (1)
	contentHeight := m.height - 6
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Build content with fixed height to prevent layout shift
	var content string
	if m.activeView == ViewSessions {
		content = m.list.View()
	} else {
		// Add blank line to match list's internal titleView padding
		content = "\n" + m.msgView.View()
	}

	// Ensure consistent height
	content = lipgloss.NewStyle().Height(contentHeight).Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content)
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
