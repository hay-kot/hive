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
const currentRepoIndicator = "◆"

// TreeItem represents an item in the tree view.
// It can be either a repo header or a session entry.
type TreeItem struct {
	// IsHeader indicates this is a repo header, not a session.
	IsHeader bool

	// Header fields (only used when IsHeader is true)
	RepoName      string
	IsCurrentRepo bool

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
		// Add header
		header := TreeItem{
			IsHeader:      true,
			RepoName:      group.Name,
			IsCurrentRepo: group.Remote == localRemote,
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

		TreeLine:       lipgloss.NewStyle().Foreground(colorGray),
		SessionName:    lipgloss.NewStyle().Foreground(colorWhite),
		SessionBranch:  lipgloss.NewStyle().Foreground(colorGray),
		SessionID:      lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")), // purple
		StatusActive:   lipgloss.NewStyle().Foreground(colorGreen),
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
	Styles       TreeDelegateStyles
	GitStatuses  *kv.Store[string, GitStatus]
	ColumnWidths *ColumnWidths
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
