package tui

import (
	"fmt"
	"image/color"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/integration/terminal"
	"github.com/hay-kot/hive/pkg/kv"
)

// Tree characters for rendering the session tree.
const (
	treeBranch = "├─"
	treeLast   = "└─"
)

// Status indicators for sessions.
const (
	statusActive   = "[●]" // green - agent actively working
	statusApproval = "[!]" // yellow - needs approval/permission
	statusReady    = "[>]" // cyan - ready for next input
	statusUnknown  = "[?]" // dim - no terminal found
	statusRecycled = "[○]" // gray - session recycled
)

// Animation constants.
const (
	// AnimationFrameCount is the total number of frames in the fade animation.
	AnimationFrameCount = 12
)

// activeAnimationColors are the colors for a subtle pulse animation.
// Uses a narrow range for a gentle breathing effect.
var activeAnimationColors = []color.Color{
	lipgloss.Color("#9ece6a"), // bright green (frame 0)
	lipgloss.Color("#93c761"), // slightly dimmer
	lipgloss.Color("#8bc058"), // dimmer
	lipgloss.Color("#84b94f"), // even dimmer
	lipgloss.Color("#7db246"), // dim
	lipgloss.Color("#76ab3d"), // dimmest (frame 5)
	lipgloss.Color("#76ab3d"), // dimmest (frame 6) - hold
	lipgloss.Color("#7db246"), // brightening
	lipgloss.Color("#84b94f"), // brighter
	lipgloss.Color("#8bc058"), // even brighter
	lipgloss.Color("#93c761"), // almost bright
	lipgloss.Color("#9ece6a"), // bright green (frame 11)
}

// Star indicator for current repository.
const currentRepoIndicator = "◆"

// renderStatusIndicator returns the styled status indicator for a session.
// For active sessions with terminal integration, it uses terminal status.
// For recycled sessions or when no terminal status is available, it falls back to session state.
// The animFrame parameter controls the fade animation for active status (0 to AnimationFrameCount-1).
func renderStatusIndicator(state session.State, termStatus *TerminalStatus, styles TreeDelegateStyles, animFrame int) string {
	// Recycled sessions always show recycled indicator
	if state == session.StateRecycled {
		return styles.StatusRecycled.Render(statusRecycled)
	}

	// If we have terminal status for active sessions, use it
	if state == session.StateActive && termStatus != nil {
		switch termStatus.Status {
		case terminal.StatusActive:
			return renderActiveIndicator(animFrame)
		case terminal.StatusApproval:
			return styles.StatusApproval.Render(statusApproval)
		case terminal.StatusReady:
			return styles.StatusReady.Render(statusReady)
		case terminal.StatusMissing:
			return styles.StatusUnknown.Render(statusUnknown)
		}
	}

	// Default: active session without terminal status shows as unknown
	// We only show active (green) when we have positive confirmation of activity
	if state == session.StateActive {
		return styles.StatusUnknown.Render(statusUnknown)
	}

	return styles.StatusRecycled.Render(statusRecycled)
}

// renderActiveIndicator renders the active status with fade animation.
func renderActiveIndicator(frame int) string {
	// Ensure frame is in bounds
	if frame < 0 || frame >= len(activeAnimationColors) {
		frame = 0
	}
	style := lipgloss.NewStyle().Foreground(activeAnimationColors[frame])
	return style.Render(statusActive)
}

// TreeItem represents an item in the tree view.
// It can be either a repo header, a session entry, or a recycled placeholder.
type TreeItem struct {
	// IsHeader indicates this is a repo header, not a session.
	IsHeader bool

	// Header fields (only used when IsHeader is true)
	RepoName      string
	IsCurrentRepo bool

	// Session fields (only used when IsHeader is false and IsRecycledPlaceholder is false)
	Session      session.Session
	IsLastInRepo bool   // Used to render └─ vs ├─
	RepoPrefix   string // The repo name for filtering purposes

	// Recycled placeholder fields (only used when IsRecycledPlaceholder is true)
	IsRecycledPlaceholder bool
	RecycledCount         int
}

// FilterValue returns the value used for filtering.
// Headers are not filterable (return empty).
// Sessions return "repoName sessionName" to allow searching by either.
// Recycled placeholders return "repoName recycled" to allow filtering.
func (i TreeItem) FilterValue() string {
	if i.IsHeader {
		return ""
	}
	if i.IsRecycledPlaceholder {
		return i.RepoPrefix + " recycled"
	}
	return i.RepoPrefix + " " + i.Session.Name
}

// BuildTreeItems converts repo groups into tree items for the list.
func BuildTreeItems(groups []RepoGroup, localRemote string) []list.Item {
	if len(groups) == 0 {
		return nil
	}

	items := make([]list.Item, 0)

	for _, group := range groups {
		// Add header
		header := TreeItem{
			IsHeader:      true,
			RepoName:      group.Name,
			IsCurrentRepo: group.Remote == localRemote,
		}
		items = append(items, header)

		// Determine if recycled placeholder will be the last item
		hasRecycled := group.RecycledCount > 0

		// Add active sessions
		for idx, s := range group.Sessions {
			isLast := idx == len(group.Sessions)-1 && !hasRecycled
			item := TreeItem{
				IsHeader:     false,
				Session:      s,
				IsLastInRepo: isLast,
				RepoPrefix:   group.Name,
			}
			items = append(items, item)
		}

		// Add recycled placeholder if there are recycled sessions
		if hasRecycled {
			placeholder := TreeItem{
				IsRecycledPlaceholder: true,
				RecycledCount:         group.RecycledCount,
				IsLastInRepo:          true,
				RepoPrefix:            group.Name,
			}
			items = append(items, placeholder)
		}
	}

	return items
}

// TreeDelegateStyles defines the styles for the tree delegate.
type TreeDelegateStyles struct {
	// Header styles
	HeaderNormal   lipgloss.Style
	HeaderSelected lipgloss.Style
	HeaderStar     lipgloss.Style

	// Session styles
	TreeLine       lipgloss.Style
	SessionName    lipgloss.Style
	SessionBranch  lipgloss.Style
	SessionID      lipgloss.Style
	StatusActive   lipgloss.Style
	StatusApproval lipgloss.Style
	StatusReady    lipgloss.Style
	StatusUnknown  lipgloss.Style
	StatusRecycled lipgloss.Style

	// Selection styles
	Selected       lipgloss.Style
	SelectedBorder lipgloss.Style
	FilterMatch    lipgloss.Style
	SelectedMatch  lipgloss.Style
}

// DefaultTreeDelegateStyles returns the default styles for tree rendering.
func DefaultTreeDelegateStyles() TreeDelegateStyles {
	return TreeDelegateStyles{
		HeaderNormal:   lipgloss.NewStyle().Bold(true).Foreground(colorWhite),
		HeaderSelected: lipgloss.NewStyle().Bold(true).Foreground(colorBlue),
		HeaderStar:     lipgloss.NewStyle().Foreground(colorYellow),

		TreeLine:       lipgloss.NewStyle().Foreground(colorGray),
		SessionName:    lipgloss.NewStyle().Foreground(colorWhite),
		SessionBranch:  lipgloss.NewStyle().Foreground(colorGray),
		SessionID:      lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")), // purple
		StatusActive:   lipgloss.NewStyle().Foreground(colorGreen),
		StatusApproval: lipgloss.NewStyle().Foreground(colorYellow),
		StatusReady:    lipgloss.NewStyle().Foreground(colorCyan),
		StatusUnknown:  lipgloss.NewStyle().Foreground(colorGray).Faint(true),
		StatusRecycled: lipgloss.NewStyle().Foreground(colorGray),

		Selected:       lipgloss.NewStyle().Foreground(colorBlue).Bold(true),
		SelectedBorder: lipgloss.NewStyle().Foreground(colorBlue),
		FilterMatch:    lipgloss.NewStyle().Underline(true),
		SelectedMatch:  lipgloss.NewStyle().Underline(true).Foreground(colorBlue).Bold(true),
	}
}

// RenderRepoHeader renders a repository header line.
func RenderRepoHeader(item TreeItem, isSelected bool, styles TreeDelegateStyles) string {
	// Repo name
	nameStyle := styles.HeaderNormal
	if isSelected {
		nameStyle = styles.HeaderSelected
	}
	result := nameStyle.Render(item.RepoName)

	// Append indicator for current repo
	if item.IsCurrentRepo {
		result += " " + styles.HeaderStar.Render(currentRepoIndicator)
	}

	return result
}

// RenderSessionLine renders a session entry with tree prefix.
func RenderSessionLine(item TreeItem, isSelected bool, gitBranch string, termStatus *TerminalStatus, styles TreeDelegateStyles, animFrame int) string {
	// Tree prefix
	var prefix string
	if item.IsLastInRepo {
		prefix = treeLast
	} else {
		prefix = treeBranch
	}
	prefixStyled := styles.TreeLine.Render(prefix)

	// Status indicator - use terminal status for active sessions
	statusStr := renderStatusIndicator(item.Session.State, termStatus, styles, animFrame)

	// Session name
	nameStyle := styles.SessionName
	if isSelected {
		nameStyle = styles.Selected
	}
	name := nameStyle.Render(item.Session.Name)

	// Branch (from git status or fallback)
	branch := ""
	if gitBranch != "" {
		branch = styles.SessionBranch.Render(" (" + gitBranch + ")")
	}

	// Short ID (last 4 chars of session ID)
	shortID := item.Session.ID
	if len(shortID) > 4 {
		shortID = shortID[len(shortID)-4:]
	}
	id := styles.SessionID.Render(" #" + shortID)

	return fmt.Sprintf("%s %s %s%s%s", prefixStyled, statusStr, name, branch, id)
}

// ColumnWidths holds the calculated widths for aligned columns.
type ColumnWidths struct {
	Name   int
	Branch int
	ID     int
}

// CalculateColumnWidths calculates the maximum widths for each column within a repo group.
func CalculateColumnWidths(sessions []session.Session, gitBranches map[string]string) ColumnWidths {
	var widths ColumnWidths

	for _, s := range sessions {
		if len(s.Name) > widths.Name {
			widths.Name = len(s.Name)
		}

		branch := gitBranches[s.Path]
		if len(branch) > widths.Branch {
			widths.Branch = len(branch)
		}

		shortID := s.ID
		if len(shortID) > 4 {
			shortID = shortID[len(shortID)-4:]
		}
		if len(shortID) > widths.ID {
			widths.ID = len(shortID)
		}
	}

	return widths
}

// PadRight pads a string to the right with spaces to reach the desired width.
func PadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// TreeDelegate handles rendering of tree items in the list.
type TreeDelegate struct {
	Styles           TreeDelegateStyles
	GitStatuses      *kv.Store[string, GitStatus]
	TerminalStatuses *kv.Store[string, TerminalStatus]
	ColumnWidths     *ColumnWidths
	AnimationFrame   int // Current frame for status animations
}

// NewTreeDelegate creates a new tree delegate with default styles.
func NewTreeDelegate() TreeDelegate {
	return TreeDelegate{
		Styles: DefaultTreeDelegateStyles(),
	}
}

// Height returns the height of each item.
// Headers are 1 line, sessions are 1 line.
func (d TreeDelegate) Height() int {
	return 1
}

// Spacing returns the spacing between items.
func (d TreeDelegate) Spacing() int {
	return 0
}

// Update handles item updates.
func (d TreeDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

// Render renders a single tree item.
func (d TreeDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	treeItem, ok := item.(TreeItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	// Build the line content
	var line string
	switch {
	case treeItem.IsHeader:
		line = d.renderHeader(treeItem, isSelected, m, index)
	case treeItem.IsRecycledPlaceholder:
		line = d.renderRecycledPlaceholder(treeItem, isSelected)
	default:
		line = d.renderSession(treeItem, isSelected, m, index)
	}

	// Selection indicator
	var prefix string
	if isSelected {
		prefix = d.Styles.SelectedBorder.Render("┃") + " "
	} else {
		prefix = "  "
	}

	_, _ = fmt.Fprintf(w, "%s%s", prefix, line)
}

// renderHeader renders a repository header.
func (d TreeDelegate) renderHeader(item TreeItem, isSelected bool, _ list.Model, _ int) string {
	// Repo name
	nameStyle := d.Styles.HeaderNormal
	if isSelected {
		nameStyle = d.Styles.HeaderSelected
	}
	result := nameStyle.Render(item.RepoName)

	// Append indicator for current repo
	if item.IsCurrentRepo {
		result += " " + d.Styles.HeaderStar.Render(currentRepoIndicator)
	}

	return result
}

// renderRecycledPlaceholder renders the collapsed recycled sessions placeholder.
func (d TreeDelegate) renderRecycledPlaceholder(item TreeItem, isSelected bool) string {
	// Tree prefix
	var prefix string
	if item.IsLastInRepo {
		prefix = treeLast
	} else {
		prefix = treeBranch
	}
	prefixStyled := d.Styles.TreeLine.Render(prefix)

	// Status indicator (recycled)
	statusStr := d.Styles.StatusRecycled.Render(statusRecycled)

	// Label with count
	labelStyle := d.Styles.StatusRecycled
	if isSelected {
		labelStyle = d.Styles.Selected
	}
	label := labelStyle.Render(fmt.Sprintf("Recycled (%d)", item.RecycledCount))

	return fmt.Sprintf("%s %s %s", prefixStyled, statusStr, label)
}

// renderSession renders a session entry.
func (d TreeDelegate) renderSession(item TreeItem, isSelected bool, m list.Model, index int) string {
	// Tree prefix
	var prefix string
	if item.IsLastInRepo {
		prefix = treeLast
	} else {
		prefix = treeBranch
	}
	prefixStyled := d.Styles.TreeLine.Render(prefix)

	// Get terminal status if available
	var termStatus *TerminalStatus
	if d.TerminalStatuses != nil {
		if ts, ok := d.TerminalStatuses.Get(item.Session.ID); ok {
			termStatus = &ts
		}
	}

	// Status indicator - use terminal status for active sessions
	statusStr := renderStatusIndicator(item.Session.State, termStatus, d.Styles, d.AnimationFrame)

	// Session name with filter matching
	nameStyle := d.Styles.SessionName
	matchStyle := d.Styles.FilterMatch
	if isSelected {
		nameStyle = d.Styles.Selected
		matchStyle = d.Styles.SelectedMatch
	}

	// Get filter matches
	matches := m.MatchesForItem(index)
	matchSet := make(map[int]bool, len(matches))
	for _, idx := range matches {
		matchSet[idx] = true
	}

	// FilterValue is "repoName sessionName", so name offset is len(repoName)+1
	nameOffset := len([]rune(item.RepoPrefix)) + 1
	name := d.renderWithMatches(item.Session.Name, nameOffset, matchSet, nameStyle, matchStyle)

	// Pad name to align columns (add spaces after styled name)
	namePadding := ""
	if d.ColumnWidths != nil && d.ColumnWidths.Name > 0 {
		padLen := d.ColumnWidths.Name - len(item.Session.Name)
		if padLen > 0 {
			namePadding = strings.Repeat(" ", padLen)
		}
	}

	// Short ID
	shortID := item.Session.ID
	if len(shortID) > 4 {
		shortID = shortID[len(shortID)-4:]
	}
	id := d.Styles.SessionID.Render(" #" + shortID)

	// Git status: branch, diff stats, clean/dirty indicator
	gitInfo := d.renderGitStatus(item.Session.Path)

	return fmt.Sprintf("%s %s %s%s%s%s", prefixStyled, statusStr, name, namePadding, id, gitInfo)
}

// renderGitStatus returns the formatted git status for a session path.
func (d TreeDelegate) renderGitStatus(path string) string {
	if d.GitStatuses == nil {
		return gitLoadingStyle.Render(" ...")
	}

	status, ok := d.GitStatuses.Get(path)
	if !ok || status.IsLoading {
		return gitLoadingStyle.Render(" ...")
	}

	if status.Error != nil {
		return ""
	}

	// Format: (branch) +N -N • clean/dirty
	branch := d.Styles.SessionBranch.Render(" (" + status.Branch + ")")
	additions := gitAdditionsStyle.Render(fmt.Sprintf(" +%d", status.Additions))
	deletions := gitDeletionsStyle.Render(fmt.Sprintf(" -%d", status.Deletions))

	var indicator string
	if status.HasChanges {
		indicator = gitDirtyStyle.Render(" • uncommitted")
	} else {
		indicator = gitCleanStyle.Render(" • clean")
	}

	return branch + additions + deletions + indicator
}

// renderWithMatches renders text with underlined characters at matched positions.
func (d TreeDelegate) renderWithMatches(text string, offset int, matchSet map[int]bool, baseStyle, matchStyle lipgloss.Style) string {
	if len(matchSet) == 0 {
		return baseStyle.Render(text)
	}

	runes := []rune(text)
	var result string
	for i, r := range runes {
		if matchSet[offset+i] {
			result += matchStyle.Render(string(r))
		} else {
			result += baseStyle.Render(string(r))
		}
	}
	return result
}
