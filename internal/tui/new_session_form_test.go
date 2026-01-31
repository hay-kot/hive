package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNewSessionForm(t *testing.T) {
	repos := []DiscoveredRepo{
		{Path: "/code/alpha", Name: "alpha", Remote: "git@github.com:user/alpha.git"},
		{Path: "/code/beta", Name: "beta", Remote: "git@github.com:user/beta.git"},
		{Path: "/code/gamma", Name: "gamma", Remote: "git@github.com:user/gamma.git"},
	}

	t.Run("creates form with repos", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		require.NotNil(t, form)
		require.NotNil(t, form.Form())
		assert.False(t, form.Submitted())
		assert.False(t, form.Cancelled())
	})

	t.Run("preselects matching remote", func(t *testing.T) {
		form := NewNewSessionForm(repos, "git@github.com:user/beta.git", nil)
		assert.Equal(t, 1, form.selectedIdx)
	})

	t.Run("defaults to first repo when no match", func(t *testing.T) {
		form := NewNewSessionForm(repos, "git@github.com:user/unknown.git", nil)
		assert.Equal(t, 0, form.selectedIdx)
	})

	t.Run("result returns selected repo", func(t *testing.T) {
		form := NewNewSessionForm(repos, "git@github.com:user/gamma.git", nil)
		form.sessionName = "my-session"
		form.SetSubmitted()

		result := form.Result()
		assert.Equal(t, "gamma", result.Repo.Name)
		assert.Equal(t, "my-session", result.SessionName)
	})

	t.Run("tracks submitted state", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		assert.False(t, form.Submitted())
		form.SetSubmitted()
		assert.True(t, form.Submitted())
	})

	t.Run("tracks cancelled state", func(t *testing.T) {
		form := NewNewSessionForm(repos, "", nil)
		assert.False(t, form.Cancelled())
		form.SetCancelled()
		assert.True(t, form.Cancelled())
	})
}
