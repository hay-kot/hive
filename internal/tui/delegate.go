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
	return 4
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

	// Build the title line: ID  Name
	title := fmt.Sprintf("%-8s %s", s.ID, s.Name)

	// Build the description line: State | Path
	var stateStyle lipgloss.Style
	switch s.State {
	case session.StateActive:
		stateStyle = d.Styles.Active
	case session.StateRecycled:
		stateStyle = d.Styles.Recycled
	default:
		stateStyle = d.Styles.Normal
	}

	state := stateStyle.Render(string(s.State))
	desc := fmt.Sprintf("%s  %s", state, s.Path)

	// Build git status line
	gitLine := d.renderGitStatus(s.Path)

	// Apply selection styling
	var titleStyle lipgloss.Style
	if isSelected {
		titleStyle = d.Styles.Selected
		title = "> " + title
	} else {
		titleStyle = d.Styles.Normal
		title = "  " + title
	}

	// Write to output
	_, _ = fmt.Fprintf(w, "%s\n", titleStyle.Render(title))
	_, _ = fmt.Fprintf(w, "  %s\n", desc)
	_, _ = fmt.Fprintf(w, "  %s", gitLine)
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

	// Format: branch +N -N • clean/uncommitted changes
	branch := gitBranchStyle.Render(status.Branch)

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
