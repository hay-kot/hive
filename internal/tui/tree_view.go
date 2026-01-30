package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/pkg/kv"
)

// Tree characters for rendering the session tree.
const (
	treeBranch = "├─"
	treeLast   = "└─"
)

// Status indicators for sessions.
const (
	statusActive   = "[●]"
	statusRecycled = "[○]"
)

// Star indicator for current repository.
const starIndicator = "★"

// TreeItem represents an item in the tree view.
// It can be either a repo header or a session entry.
type TreeItem struct {
	// IsHeader indicates this is a repo header, not a session.
	IsHeader bool

	// Header fields (only used when IsHeader is true)
	RepoName      string
	IsCurrentRepo bool
	TotalCount    int
	RecycledCount int

	// Session fields (only used when IsHeader is false)
	Session      session.Session
	IsLastInRepo bool   // Used to render └─ vs ├─
	RepoPrefix   string // The repo name for filtering purposes
}

// FilterValue returns the value used for filtering.
// Headers are not filterable (return empty).
// Sessions return "repoName sessionName" to allow searching by either.
func (i TreeItem) FilterValue() string {
	if i.IsHeader {
		return ""
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
		// Count recycled sessions
		recycledCount := 0
		for _, s := range group.Sessions {
			if s.State == session.StateRecycled {
				recycledCount++
			}
		}

		// Add header
		header := TreeItem{
			IsHeader:      true,
			RepoName:      group.Name,
			IsCurrentRepo: group.Remote == localRemote,
			TotalCount:    len(group.Sessions),
			RecycledCount: recycledCount,
		}
		items = append(items, header)

		// Add sessions
		for idx, s := range group.Sessions {
			item := TreeItem{
				IsHeader:     false,
				Session:      s,
				IsLastInRepo: idx == len(group.Sessions)-1,
				RepoPrefix:   group.Name,
			}
			items = append(items, item)
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
	HeaderCount    lipgloss.Style

	// Session styles
	TreeLine       lipgloss.Style
	SessionName    lipgloss.Style
	SessionBranch  lipgloss.Style
	SessionID      lipgloss.Style
	StatusActive   lipgloss.Style
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
		HeaderCount:    lipgloss.NewStyle().Foreground(colorGray),

		TreeLine:       lipgloss.NewStyle().Foreground(colorGray),
		SessionName:    lipgloss.NewStyle().Foreground(colorWhite),
		SessionBranch:  lipgloss.NewStyle().Foreground(colorGray),
		SessionID:      lipgloss.NewStyle().Foreground(colorGray),
		StatusActive:   lipgloss.NewStyle().Foreground(colorGreen),
		StatusRecycled: lipgloss.NewStyle().Foreground(colorYellow),

		Selected:       lipgloss.NewStyle().Foreground(colorBlue).Bold(true),
		SelectedBorder: lipgloss.NewStyle().Foreground(colorBlue),
		FilterMatch:    lipgloss.NewStyle().Underline(true),
		SelectedMatch:  lipgloss.NewStyle().Underline(true).Foreground(colorBlue).Bold(true),
	}
}

// RenderRepoHeader renders a repository header line.
func RenderRepoHeader(item TreeItem, isSelected bool, styles TreeDelegateStyles) string {
	var parts []string

	// Star indicator for current repo
	if item.IsCurrentRepo {
		parts = append(parts, styles.HeaderStar.Render(starIndicator))
	}

	// Repo name
	nameStyle := styles.HeaderNormal
	if isSelected {
		nameStyle = styles.HeaderSelected
	}
	parts = append(parts, nameStyle.Render(item.RepoName))

	// Count indicator: (N sessions, M recycled) or just (N sessions)
	var countStr string
	if item.RecycledCount > 0 {
		countStr = fmt.Sprintf("(%d sessions, %d recycled)", item.TotalCount, item.RecycledCount)
	} else {
		countStr = fmt.Sprintf("(%d sessions)", item.TotalCount)
	}
	parts = append(parts, styles.HeaderCount.Render(countStr))

	return strings.Join(parts, " ")
}

// RenderSessionLine renders a session entry with tree prefix.
func RenderSessionLine(item TreeItem, isSelected bool, gitBranch string, styles TreeDelegateStyles) string {
	// Tree prefix
	var prefix string
	if item.IsLastInRepo {
		prefix = treeLast
	} else {
		prefix = treeBranch
	}
	prefixStyled := styles.TreeLine.Render(prefix)

	// Status indicator
	var statusStr string
	if item.Session.State == session.StateActive {
		statusStr = styles.StatusActive.Render(statusActive)
	} else {
		statusStr = styles.StatusRecycled.Render(statusRecycled)
	}

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
	Styles      TreeDelegateStyles
	GitStatuses *kv.Store[string, GitStatus]
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
	if treeItem.IsHeader {
		line = d.renderHeader(treeItem, isSelected, m, index)
	} else {
		line = d.renderSession(treeItem, isSelected, m, index)
	}

	// Selection indicator
	var prefix string
	if isSelected {
		prefix = d.Styles.SelectedBorder.Render("▸") + " "
	} else {
		prefix = "  "
	}

	_, _ = fmt.Fprintf(w, "%s%s", prefix, line)
}

// renderHeader renders a repository header.
func (d TreeDelegate) renderHeader(item TreeItem, isSelected bool, m list.Model, _ int) string {
	var parts []string

	// Star indicator for current repo
	if item.IsCurrentRepo {
		parts = append(parts, d.Styles.HeaderStar.Render(starIndicator))
	}

	// Repo name with filter highlighting
	nameStyle := d.Styles.HeaderNormal
	if isSelected {
		nameStyle = d.Styles.HeaderSelected
	}
	parts = append(parts, nameStyle.Render(item.RepoName))

	// Count indicator
	var countStr string
	if item.RecycledCount > 0 {
		countStr = fmt.Sprintf("(%d sessions, %d recycled)", item.TotalCount, item.RecycledCount)
	} else {
		countStr = fmt.Sprintf("(%d sessions)", item.TotalCount)
	}
	parts = append(parts, d.Styles.HeaderCount.Render(countStr))

	_ = m // unused but kept for consistency with session rendering

	return strings.Join(parts, " ")
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

	// Status indicator
	var statusStr string
	if item.Session.State == session.StateActive {
		statusStr = d.Styles.StatusActive.Render(statusActive)
	} else {
		statusStr = d.Styles.StatusRecycled.Render(statusRecycled)
	}

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

	// Git branch from status
	branch := ""
	if d.GitStatuses != nil {
		if status, ok := d.GitStatuses.Get(item.Session.Path); ok && !status.IsLoading && status.Error == nil {
			branch = d.Styles.SessionBranch.Render(" (" + status.Branch + ")")
		}
	}

	// Short ID
	shortID := item.Session.ID
	if len(shortID) > 4 {
		shortID = shortID[len(shortID)-4:]
	}
	id := d.Styles.SessionID.Render(" #" + shortID)

	return fmt.Sprintf("%s %s %s%s%s", prefixStyled, statusStr, name, branch, id)
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
