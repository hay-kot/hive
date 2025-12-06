package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/pkg/kv"
)

// maxPromptDisplayLen is the maximum number of runes to display for a session prompt.
const maxPromptDisplayLen = 60

// SessionItem wraps a session for the list component.
type SessionItem struct {
	Session session.Session
}

// FilterValue returns the value used for filtering.
// Format: "repoName sessionName" to allow searching by either.
func (i SessionItem) FilterValue() string {
	repoName := extractRepoName(i.Session.Remote)
	return repoName + " " + i.Session.Name
}

// SessionDelegate handles rendering of session items in the list.
type SessionDelegate struct {
	Styles      SessionDelegateStyles
	GitStatuses *kv.Store[string, GitStatus]
}

// SessionDelegateStyles defines the styles for the delegate.
type SessionDelegateStyles struct {
	Normal        lipgloss.Style
	Selected      lipgloss.Style
	Active        lipgloss.Style
	Recycled      lipgloss.Style
	FilterMatch   lipgloss.Style
	SelectedMatch lipgloss.Style
}

// DefaultSessionDelegateStyles returns the default styles.
func DefaultSessionDelegateStyles() SessionDelegateStyles {
	return SessionDelegateStyles{
		Normal:        normalStyle,
		Selected:      selectedStyle,
		Active:        activeStyle,
		Recycled:      recycledStyle,
		FilterMatch:   lipgloss.NewStyle().Underline(true),
		SelectedMatch: lipgloss.NewStyle().Underline(true).Foreground(colorBlue).Bold(true),
	}
}

// NewSessionDelegate creates a new session delegate with default styles.
func NewSessionDelegate() SessionDelegate {
	return SessionDelegate{
		Styles: DefaultSessionDelegateStyles(),
	}
}

// Height returns the height of each item.
func (d SessionDelegate) Height() int {
	return 5
}

// Spacing returns the spacing between items.
func (d SessionDelegate) Spacing() int {
	return 1
}

// Update handles item updates.
func (d SessionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

// Render renders a single item.
func (d SessionDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	sessionItem, ok := item.(SessionItem)
	if !ok {
		return
	}

	s := sessionItem.Session
	isSelected := index == m.Index()

	// Build the title line: Name [state]
	var stateStyle lipgloss.Style
	switch s.State {
	case session.StateActive:
		stateStyle = d.Styles.Active
	case session.StateRecycled:
		stateStyle = d.Styles.Recycled
	default:
		stateStyle = d.Styles.Normal
	}

	stateTag := stateStyle.Render(fmt.Sprintf("[%s]", s.State))

	// Determine styles based on selection
	var nameStyle, matchStyle lipgloss.Style
	if isSelected {
		nameStyle = d.Styles.Selected
		matchStyle = d.Styles.SelectedMatch
	} else {
		nameStyle = d.Styles.Normal
		matchStyle = d.Styles.FilterMatch
	}

	// Get filter matches for highlighting
	matches := m.MatchesForItem(index)
	matchSet := make(map[int]bool, len(matches))
	for _, idx := range matches {
		matchSet[idx] = true
	}

	// Build title: <git icon> <repo> • <name> [state]
	// FilterValue format is "repoName sessionName", so we map indices accordingly
	var title string
	repoName := extractRepoName(s.Remote)
	repoOffset := 0
	nameOffset := len([]rune(repoName)) + 1 // +1 for space separator

	if s.Remote != "" {
		styledRepo := d.renderWithMatches(repoName, repoOffset, matchSet, nameStyle, matchStyle)
		repoPrefix := iconGit + "  " + styledRepo
		styledName := d.renderWithMatches(s.Name, nameOffset, matchSet, nameStyle, matchStyle)
		title = fmt.Sprintf("%s %s %s %s", repoPrefix, iconDot, styledName, stateTag)
	} else {
		styledName := d.renderWithMatches(s.Name, nameOffset, matchSet, nameStyle, matchStyle)
		title = fmt.Sprintf("%s %s", styledName, stateTag)
	}

	// Build the description line: Path
	path := pathStyle.Render(s.Path)

	// Build prompt line (truncated with ellipsis)
	prompt := s.Prompt
	if prompt == "" {
		prompt = "(no prompt)"
	}
	promptRunes := []rune(prompt)
	if len(promptRunes) > maxPromptDisplayLen {
		prompt = string(promptRunes[:maxPromptDisplayLen-3]) + "..."
	}
	promptLine := promptStyle.Render(prompt)

	// Build git status line
	gitLine := d.renderGitStatus(s.Path)

	// Apply selection styling with left border
	var border string
	if isSelected {
		border = selectedBorderStyle.Render("┃") + " "
	} else {
		border = "  "
	}

	// Write to output with left border
	_, _ = fmt.Fprintf(w, "%s%s\n", border, title)
	_, _ = fmt.Fprintf(w, "%s%s\n", border, promptLine)
	_, _ = fmt.Fprintf(w, "%s%s\n", border, path)
	_, _ = fmt.Fprintf(w, "%s%s", border, gitLine)
}

// renderWithMatches renders text with underlined characters at matched positions.
// offset is the starting position of this text within the FilterValue string.
func (d SessionDelegate) renderWithMatches(text string, offset int, matchSet map[int]bool, baseStyle, matchStyle lipgloss.Style) string {
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

// renderGitStatus returns the formatted git status line for a session path.
func (d SessionDelegate) renderGitStatus(path string) string {
	if d.GitStatuses == nil {
		return gitLoadingStyle.Render("Loading git status...")
	}

	status, ok := d.GitStatuses.Get(path)
	if !ok || status.IsLoading {
		return gitLoadingStyle.Render("Loading git status...")
	}

	if status.Error != nil {
		return gitLoadingStyle.Render("Git: unavailable")
	}

	// Format:  branch +N -N • clean/uncommitted changes
	branch := gitBranchStyle.Render("\ue725 " + status.Branch)

	// Diff stats with colored additions (green) and deletions (red)
	additions := gitAdditionsStyle.Render(fmt.Sprintf(" +%d", status.Additions))
	deletions := gitDeletionsStyle.Render(fmt.Sprintf(" -%d", status.Deletions))

	// Clean/dirty indicator
	var indicator string
	if status.HasChanges {
		indicator = gitDirtyStyle.Render(" • uncommitted changes")
	} else {
		indicator = gitCleanStyle.Render(" • clean")
	}

	return branch + additions + deletions + indicator
}
