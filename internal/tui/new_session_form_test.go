package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyPress creates a tea.KeyPressMsg for testing.
func keyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func TestNewNewSessionForm(t *testing.T) {
	repos := []DiscoveredRepo{
		{Path: "/code/alpha", Name: "alpha", Remote: "git@github.com:user/alpha.git"},
		{Path: "/code/beta", Name: "beta", Remote: "git@github.com:user/beta.git"},
		{Path: "/code/gamma", Name: "gamma", Remote: "git@github.com:user/gamma.git"},
	}

	t.Run("creates form with repos", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		require.NotNil(t, form)
		assert.False(t, form.Submitted())
		assert.False(t, form.Cancelled())
	})

	t.Run("preselects matching remote", func(t *testing.T) {
		form := NewNewSessionForm(repos, "git@github.com:user/beta.git", nil)
		// The second repo (index 1) should be selected
		idx := form.repoSelect.SelectedIndex()
		assert.Equal(t, 1, idx)
	})

	t.Run("defaults to first repo when no match", func(t *testing.T) {
		form := NewNewSessionForm(repos, "git@github.com:user/unknown.git", nil)
		idx := form.repoSelect.SelectedIndex()
		assert.Equal(t, 0, idx)
	})

	t.Run("result returns selected repo", func(t *testing.T) {
		form := NewNewSessionForm(repos, "git@github.com:user/gamma.git", nil)
		// Simulate typing a session name
		form.focusedField = 1 // Focus name input
		form.nameInput.SetValue("my-session")
		form.submitted = true

		result := form.Result()
		assert.Equal(t, "gamma", result.Repo.Name)
		assert.Equal(t, "my-session", result.SessionName)
	})

	t.Run("tracks submitted state", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		assert.False(t, form.Submitted())
		form.submitted = true
		assert.True(t, form.Submitted())
	})

	t.Run("tracks cancelled state", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		assert.False(t, form.Cancelled())
		form.cancelled = true
		assert.True(t, form.Cancelled())
	})

	t.Run("validates empty session name", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		form.focusedField = 1 // Focus name input
		// Empty name - try to submit
		updated, _ := form.Update(keyPress(tea.KeyEnter))
		assert.False(t, updated.Submitted())
		assert.Equal(t, "Session name is required", updated.nameError)
	})

	t.Run("validates duplicate session name", func(t *testing.T) {
		existingNames := map[string]bool{"existing-session": true}
		form := NewNewSessionForm(repos, "", existingNames)
		form.focusedField = 1 // Focus name input
		form.nameInput.SetValue("existing-session")
		// Try to submit with duplicate name
		updated, _ := form.Update(keyPress(tea.KeyEnter))
		assert.False(t, updated.Submitted())
		assert.Equal(t, "Session name already exists", updated.nameError)
	})

	t.Run("esc cancels form", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		form.focusedField = 1 // Focus name input (not filtering)
		updated, _ := form.Update(keyPress(tea.KeyEscape))
		assert.True(t, updated.Cancelled())
	})

	t.Run("tab switches focus", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		assert.Equal(t, 0, form.focusedField)
		updated, _ := form.Update(keyPress(tea.KeyTab))
		assert.Equal(t, 1, updated.focusedField)
		updated, _ = updated.Update(keyPress(tea.KeyTab))
		assert.Equal(t, 0, updated.focusedField)
	})
}
