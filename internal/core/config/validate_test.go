package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDeep_ValidConfig(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Commands: Commands{
			Spawn:   []string{"echo {{.Path}}", "echo {{.Name}} {{.Prompt}}"},
			Recycle: []string{"git reset --hard", "git checkout main"},
		},
		Hooks: []Hook{
			{Pattern: "^https://github.com/.*", Commands: []string{"echo hello"}},
		},
		Keybindings: map[string]Keybinding{
			"r": {Action: ActionRecycle, Help: "recycle"},
			"o": {Sh: "open {{.Path}}", Help: "open"},
		},
	}

	result := cfg.ValidateDeep("")
	assert.True(t, result.IsValid(), "expected valid config, got errors: %v", result.Errors)
	assert.Empty(t, result.Errors)
}

func TestValidateDeep_InvalidSpawnTemplate(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Commands: Commands{
			Spawn: []string{"echo {{.Path}", "echo {{.Invalid}}"},
		},
	}

	result := cfg.ValidateDeep("")

	assert.False(t, result.IsValid())
	assert.Len(t, result.Errors, 2)
	assert.Equal(t, "Spawn Commands", result.Errors[0].Category)
	assert.Contains(t, result.Errors[0].Message, "template error")
}

func TestValidateDeep_InvalidHookPattern(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Hooks: []Hook{
			{Pattern: "[invalid", Commands: []string{"echo"}},
		},
	}

	result := cfg.ValidateDeep("")

	assert.False(t, result.IsValid())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "Hooks", result.Errors[0].Category)
	assert.Contains(t, result.Errors[0].Message, "invalid regex")
}

func TestValidateDeep_GlobPatternWarning(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Hooks: []Hook{
			{Pattern: "github.com/**/*.git", Commands: []string{"echo"}},
		},
	}

	result := cfg.ValidateDeep("")

	// Should have error for invalid regex
	assert.False(t, result.IsValid())

	// Should also have warning about glob pattern
	hasGlobWarning := false
	for _, w := range result.Warnings {
		if w.Category == "Hooks" && contains(w.Message, "glob") {
			hasGlobWarning = true
			break
		}
	}
	assert.True(t, hasGlobWarning, "expected warning about glob pattern")
}

func TestValidateDeep_KeybindingBothActionAndSh(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"x": {Action: ActionRecycle, Sh: "echo test"},
		},
	}

	result := cfg.ValidateDeep("")

	assert.False(t, result.IsValid())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "cannot have both")
}

func TestValidateDeep_KeybindingNeitherActionNorSh(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"x": {Help: "does nothing"},
		},
	}

	result := cfg.ValidateDeep("")

	assert.False(t, result.IsValid())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "must have either")
}

func TestValidateDeep_KeybindingInvalidAction(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"x": {Action: "invalid"},
		},
	}

	result := cfg.ValidateDeep("")

	assert.False(t, result.IsValid())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "invalid action")
}

func TestValidateDeep_KeybindingInvalidShTemplate(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"o": {Sh: "open {{.Invalid}}"},
		},
	}

	result := cfg.ValidateDeep("")

	assert.False(t, result.IsValid())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "template error")
}

func TestValidateDeep_KeybindingValidShTemplate(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"o": {Sh: "open {{.Path}} {{.Remote}} {{.ID}} {{.Name}}"},
		},
	}

	result := cfg.ValidateDeep("")

	assert.True(t, result.IsValid())
}

func TestValidateDeep_GitPathNotFound(t *testing.T) {
	cfg := &Config{
		GitPath: "/nonexistent/path/to/git",
		DataDir: t.TempDir(),
	}

	result := cfg.ValidateDeep("")

	assert.False(t, result.IsValid())
	hasGitError := false
	for _, e := range result.Errors {
		if e.Item == "git_path" {
			hasGitError = true
			break
		}
	}
	assert.True(t, hasGitError, "expected error about git path")
}

func TestValidateDeep_DataDirIsFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "notadir")
	require.NoError(t, os.WriteFile(tmpFile, []byte("test"), 0o644))

	cfg := &Config{
		GitPath: "git",
		DataDir: tmpFile,
	}

	result := cfg.ValidateDeep("")

	assert.False(t, result.IsValid())
	hasDataDirError := false
	for _, e := range result.Errors {
		if e.Item == "data_dir" {
			hasDataDirError = true
			break
		}
	}
	assert.True(t, hasDataDirError, "expected error about data dir")
}

func TestValidateDeep_ConfigFileIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
	}

	result := cfg.ValidateDeep(tmpDir)

	assert.False(t, result.IsValid())
	hasConfigError := false
	for _, e := range result.Errors {
		if e.Item == "config file" {
			hasConfigError = true
			break
		}
	}
	assert.True(t, hasConfigError, "expected error about config file being a directory")
}

func TestValidateDeep_EmptyHookCommands(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Hooks: []Hook{
			{Pattern: ".*", Commands: []string{}},
		},
	}

	result := cfg.ValidateDeep("")

	// Should be valid but have a warning
	assert.True(t, result.IsValid())
	hasWarning := false
	for _, w := range result.Warnings {
		if w.Category == "Hooks" && contains(w.Message, "no commands") {
			hasWarning = true
			break
		}
	}
	assert.True(t, hasWarning, "expected warning about empty hook commands")
}

func TestValidateDeep_EmptyRecycleCommands(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Commands: Commands{
			Recycle: []string{},
		},
	}

	result := cfg.ValidateDeep("")

	// Should be valid but have a warning
	assert.True(t, result.IsValid())
	hasWarning := false
	for _, w := range result.Warnings {
		if w.Category == "Recycle Commands" {
			hasWarning = true
			break
		}
	}
	assert.True(t, hasWarning, "expected warning about empty recycle commands")
}

func TestLooksLikeGlob(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"**/*.git", true},
		{"*.txt", true},
		{"foo/**", true},
		{"^https://.*", false},
		{"github\\.com/.*", false},
		{"[a-z]+", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := looksLikeGlob(tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidationResult_IsValid(t *testing.T) {
	t.Run("empty result is valid", func(t *testing.T) {
		result := &ValidationResult{}
		assert.True(t, result.IsValid())
	})

	t.Run("result with only warnings is valid", func(t *testing.T) {
		result := &ValidationResult{
			Warnings: []ValidationWarning{{Message: "test"}},
		}
		assert.True(t, result.IsValid())
	})

	t.Run("result with errors is not valid", func(t *testing.T) {
		result := &ValidationResult{
			Errors: []ValidationError{{Message: "test"}},
		}
		assert.False(t, result.IsValid())
	})
}
