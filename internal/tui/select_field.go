package tui

import (
	"io"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// SelectItem represents an item in a SelectField.
type SelectItem struct {
	label string
	value int // index into original data
}

// FilterValue implements list.Item.
func (i SelectItem) FilterValue() string { return i.label }

// SelectField is a select input component wrapping list.Model.
type SelectField struct {
	list    list.Model
	title   string
	focused bool
	width   int
	height  int
}

// selectItemDelegate renders SelectItem in the list.
type selectItemDelegate struct{}

func (d selectItemDelegate) Height() int                             { return 1 }
func (d selectItemDelegate) Spacing() int                            { return 0 }
func (d selectItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d selectItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(SelectItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	// Style for selected vs unselected
	style := lipgloss.NewStyle().Foreground(colorWhite)
	cursor := "  "
	if isSelected {
		style = style.Foreground(colorBlue).Bold(true)
		cursor = "> "
	}

	_, _ = io.WriteString(w, cursor)
	_, _ = io.WriteString(w, style.Render(item.label))
}

// NewSelectField creates a new select field with the given items.
// selected is the index to pre-select (-1 for none).
func NewSelectField(title string, items []SelectItem, selected int) SelectField {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	delegate := selectItemDelegate{}
	// Height 8 matches the original huh config
	l := list.New(listItems, delegate, 40, 8)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.TitleBar = lipgloss.NewStyle()

	// Configure filter input styles
	l.FilterInput.Prompt = "/ "
	filterStyles := textinput.DefaultStyles(true)
	filterStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(colorBlue)
	filterStyles.Cursor.Color = colorBlue
	l.FilterInput.SetStyles(filterStyles)

	// Pre-select the item
	if selected >= 0 && selected < len(items) {
		l.Select(selected)
	}

	return SelectField{
		list:   l,
		title:  title,
		width:  40,
		height: 8,
	}
}

// SetSize sets the dimensions of the select field.
func (s *SelectField) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetSize(width-4, height) // Account for border padding
}

// Focus sets focus state on the select field.
func (s *SelectField) Focus() tea.Cmd {
	s.focused = true
	return nil
}

// Blur removes focus from the select field.
func (s *SelectField) Blur() {
	s.focused = false
}

// Focused returns whether the field is focused.
func (s *SelectField) Focused() bool {
	return s.focused
}

// SelectedIndex returns the index of the selected item.
func (s *SelectField) SelectedIndex() int {
	item := s.list.SelectedItem()
	if item == nil {
		return -1
	}
	if selectItem, ok := item.(SelectItem); ok {
		return selectItem.value
	}
	return -1
}

// IsFiltering returns whether the list is currently filtering.
func (s *SelectField) IsFiltering() bool {
	return s.list.SettingFilter()
}

// Update handles messages for the select field.
func (s SelectField) Update(msg tea.Msg) (SelectField, tea.Cmd) {
	if !s.focused {
		return s, nil
	}

	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

// KeyMap returns keys that the select field uses (for help integration).
func (s *SelectField) KeyMap() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	}
}

// View renders the select field.
func (s SelectField) View() string {
	// Title
	titleStyle := formTitleBlurredStyle
	if s.focused {
		titleStyle = formTitleStyle
	}
	title := titleStyle.Render(s.title)

	// Border style based on focus (left border only)
	borderStyle := formFieldStyle
	if s.focused {
		borderStyle = formFieldFocusedStyle
	}

	// List content - trim trailing whitespace from list view
	listView := s.list.View()

	content := borderStyle.Render(listView)

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}
