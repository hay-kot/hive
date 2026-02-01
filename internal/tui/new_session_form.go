package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// NewSessionForm manages the new session creation form.
type NewSessionForm struct {
	repos         []DiscoveredRepo
	existingNames map[string]bool

	repoSelect SelectField
	nameInput  textinput.Model

	focusedField int // 0 = repo select, 1 = name input
	submitted    bool
	cancelled    bool
	nameError    string
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
	// Find preselected index
	selectedIdx := 0
	for i, r := range repos {
		if r.Remote == preselectedRemote {
			selectedIdx = i
			break
		}
	}

	// Build select items
	items := make([]SelectItem, len(repos))
	for i, r := range repos {
		items[i] = SelectItem{
			label: r.Name,
			value: i,
		}
	}

	// Create select field
	repoSelect := NewSelectField("Repository", items, selectedIdx)
	repoSelect.Focus()

	// Create text input for session name
	nameInput := textinput.New()
	nameInput.Placeholder = "my-feature-branch"
	nameInput.CharLimit = 64
	nameInput.Prompt = "" // No prompt prefix

	// Style the input
	styles := textinput.DefaultStyles(true)
	styles.Cursor.Color = colorBlue
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(colorGray)
	styles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(colorGray)
	nameInput.SetStyles(styles)

	return &NewSessionForm{
		repos:         repos,
		existingNames: existingNames,
		repoSelect:    repoSelect,
		nameInput:     nameInput,
		focusedField:  0,
	}
}

// Init returns the initial command for the form.
func (f *NewSessionForm) Init() tea.Cmd {
	return nil
}

// Update handles messages for the form.
func (f *NewSessionForm) Update(msg tea.Msg) (NewSessionForm, tea.Cmd) {
	// Handle key messages for navigation and submission
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		return f.handleKey(keyMsg)
	}

	// Route other messages to focused field
	return f.updateFocusedField(msg)
}

// handleKey processes key events.
func (f *NewSessionForm) handleKey(msg tea.KeyMsg) (NewSessionForm, tea.Cmd) {
	key := msg.String()

	switch key {
	case "tab", "shift+tab":
		// Toggle focus between fields
		if f.focusedField == 0 {
			f.focusedField = 1
			f.repoSelect.Blur()
			return *f, f.nameInput.Focus()
		}
		f.focusedField = 0
		f.nameInput.Blur()
		return *f, f.repoSelect.Focus()

	case "enter":
		if f.focusedField == 0 {
			// On repo select, move to name input
			f.focusedField = 1
			f.repoSelect.Blur()
			return *f, f.nameInput.Focus()
		}
		// On name input, validate and submit
		return f.validateAndSubmit()

	case "esc":
		// If filtering in select, let the select handle it
		if f.focusedField == 0 && f.repoSelect.IsFiltering() {
			return f.updateFocusedField(msg)
		}
		f.cancelled = true
		return *f, nil
	}

	// Pass to focused field
	return f.updateFocusedField(msg)
}

// updateFocusedField routes messages to the currently focused field.
func (f *NewSessionForm) updateFocusedField(msg tea.Msg) (NewSessionForm, tea.Cmd) {
	var cmd tea.Cmd

	if f.focusedField == 0 {
		f.repoSelect, cmd = f.repoSelect.Update(msg)
	} else {
		f.nameInput, cmd = f.nameInput.Update(msg)
		// Clear error when typing
		f.nameError = ""
	}

	return *f, cmd
}

// validateAndSubmit validates the form and submits if valid.
func (f *NewSessionForm) validateAndSubmit() (NewSessionForm, tea.Cmd) {
	name := f.nameInput.Value()

	// Validate name
	if name == "" {
		f.nameError = "Session name is required"
		return *f, nil
	}
	if f.existingNames[name] {
		f.nameError = "Session name already exists"
		return *f, nil
	}

	f.submitted = true
	return *f, nil
}

// Submitted returns true if the form was submitted.
func (f *NewSessionForm) Submitted() bool {
	return f.submitted
}

// Cancelled returns true if the form was cancelled.
func (f *NewSessionForm) Cancelled() bool {
	return f.cancelled
}

// Result returns the form result. Only valid if Submitted() is true.
func (f *NewSessionForm) Result() NewSessionFormResult {
	idx := f.repoSelect.SelectedIndex()
	if idx < 0 || idx >= len(f.repos) {
		idx = 0
	}
	return NewSessionFormResult{
		Repo:        f.repos[idx],
		SessionName: f.nameInput.Value(),
	}
}

// View renders the form.
func (f *NewSessionForm) View() string {
	// Repo select (already has title integrated)
	repoView := f.repoSelect.View()

	// Name input section - title integrated into bordered area
	nameTitleStyle := formTitleBlurredStyle
	if f.focusedField == 1 {
		nameTitleStyle = formTitleStyle
	}
	nameTitle := nameTitleStyle.Render("Session Name")

	// Build content: title + input (+ error if present)
	nameContent := lipgloss.JoinVertical(lipgloss.Left, nameTitle, f.nameInput.View())

	// Add error inside the bordered area if present
	if f.nameError != "" {
		errorView := formErrorStyle.Render(f.nameError)
		nameContent = lipgloss.JoinVertical(lipgloss.Left, nameContent, errorView)
	}

	// Apply border style
	inputBorderStyle := formFieldStyle
	if f.focusedField == 1 {
		inputBorderStyle = formFieldFocusedStyle
	}
	nameSection := inputBorderStyle.Render(nameContent)

	// Help text
	helpText := formHelpStyle.Render("tab: switch fields • enter: submit • esc: cancel")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		repoView,
		"",
		nameSection,
		"",
		helpText,
	)
}
