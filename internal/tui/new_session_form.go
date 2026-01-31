package tui

import (
	"errors"

	"github.com/charmbracelet/huh"
	"github.com/hay-kot/hive/internal/styles"
)

// NewSessionForm wraps a huh.Form for creating new sessions.
type NewSessionForm struct {
	form        *huh.Form
	repos       []DiscoveredRepo
	selectedIdx int    // index into repos
	sessionName string // entered session name
	submitted   bool
	cancelled   bool
}

// NewSessionFormResult contains the form submission result.
type NewSessionFormResult struct {
	Repo        DiscoveredRepo
	SessionName string
}

// NewNewSessionForm creates a new session form with the given repos.
// If preselectedRemote is non-empty, the matching repo will be pre-selected.
// existingNames is used to validate that the session name is unique.
func NewNewSessionForm(repos []DiscoveredRepo, preselectedRemote string, existingNames map[string]bool) *NewSessionForm {
	f := &NewSessionForm{
		repos: repos,
	}

	// Find preselected index
	for i, r := range repos {
		if r.Remote == preselectedRemote {
			f.selectedIdx = i
			break
		}
	}

	// Build select options
	options := make([]huh.Option[int], len(repos))
	for i, r := range repos {
		options[i] = huh.NewOption(r.Name, i)
	}

	f.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Repository").
				Options(options...).
				Value(&f.selectedIdx).
				Filtering(true).
				Height(8),
			huh.NewInput().
				Title("Session Name").
				Value(&f.sessionName).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("session name is required")
					}
					if existingNames[s] {
						return errors.New("session name already exists")
					}
					return nil
				}),
		),
	).WithTheme(styles.FormTheme())

	return f
}

// Form returns the underlying huh.Form for tea.Model integration.
func (f *NewSessionForm) Form() *huh.Form {
	return f.form
}

// Submitted returns true if the form was submitted.
func (f *NewSessionForm) Submitted() bool {
	return f.submitted
}

// Cancelled returns true if the form was cancelled.
func (f *NewSessionForm) Cancelled() bool {
	return f.cancelled
}

// SetSubmitted marks the form as submitted.
func (f *NewSessionForm) SetSubmitted() {
	f.submitted = true
}

// SetCancelled marks the form as cancelled.
func (f *NewSessionForm) SetCancelled() {
	f.cancelled = true
}

// Result returns the form result. Only valid if Submitted() is true.
func (f *NewSessionForm) Result() NewSessionFormResult {
	return NewSessionFormResult{
		Repo:        f.repos[f.selectedIdx],
		SessionName: f.sessionName,
	}
}

// View renders the form.
func (f *NewSessionForm) View() string {
	return f.form.View()
}
