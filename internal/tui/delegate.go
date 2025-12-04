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
func (i SessionItem) FilterValue() string {
	return i.Session.ID + " " + i.Session.Name
}

// SessionDelegate handles rendering of session items in the list.
type SessionDelegate struct {
	Styles      SessionDelegateStyles
	GitStatuses *kv.Store[string, GitStatus]
}

// SessionDelegateStyles defines the styles for the delegate.
type SessionDelegateStyles struct {
	Normal   lipgloss.Style
	Selected lipgloss.Style
	Active   lipgloss.Style
	Recycled lipgloss.Style
}

// DefaultSessionDelegateStyles returns the default styles.
func DefaultSessionDelegateStyles() SessionDelegateStyles {
	return SessionDelegateStyles{
		Normal:   normalStyle,
		Selected: selectedStyle,
		Active:   activeStyle,
		Recycled: recycledStyle,
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
	title := fmt.Sprintf("%s %s", s.Name, stateTag)

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
	var titleStyle lipgloss.Style
	var border string
	if isSelected {
		titleStyle = d.Styles.Selected
		border = selectedBorderStyle.Render("┃") + " "
	} else {
		titleStyle = d.Styles.Normal
		border = "  "
	}

	// Write to output with left border
	_, _ = fmt.Fprintf(w, "%s%s\n", border, titleStyle.Render(title))
	_, _ = fmt.Fprintf(w, "%s%s\n", border, promptLine)
	_, _ = fmt.Fprintf(w, "%s%s\n", border, path)
	_, _ = fmt.Fprintf(w, "%s%s", border, gitLine)
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
