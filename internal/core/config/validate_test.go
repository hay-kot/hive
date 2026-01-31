package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hay-kot/criterio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig returns a Config with all required fields set for testing.
func validConfig(t *testing.T) *Config {
	t.Helper()
	return &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Git:     GitConfig{StatusWorkers: 1},
	}
}

func TestValidateDeep_ValidConfig(t *testing.T) {
	cfg := validConfig(t)
	cfg.Commands = Commands{
		Spawn:      []string{"echo {{.Path}}", "echo {{.Name}} {{.Slug}}"},
		BatchSpawn: []string{"echo {{.Path}}", "echo {{.Name}} {{.Prompt}}"},
		Recycle:    []string{"git reset --hard", "git checkout main"},
	}
	cfg.Rules = []Rule{
		{Pattern: "^https://github.com/.*", Commands: []string{"echo hello"}},
	}
	cfg.Keybindings = map[string]Keybinding{
		"r": {Action: ActionRecycle, Help: "recycle"},
		"o": {Sh: "open {{.Path}}", Help: "open"},
	}

	err := cfg.ValidateDeep("")
	assert.NoError(t, err, "expected valid config")
}

func TestValidateDeep_InvalidSpawnTemplate(t *testing.T) {
	cfg := validConfig(t)
	cfg.Commands = Commands{
		Spawn: []string{"echo {{.Path}", "echo {{.Invalid}}"},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 2)
	assert.Contains(t, fieldErrs[0].Field, "commands.spawn")
	assert.Contains(t, fieldErrs[0].Err.Error(), "template error")
}

func TestValidateDeep_InvalidRecycleTemplate(t *testing.T) {
	cfg := validConfig(t)
	cfg.Commands = Commands{
		Recycle: []string{"git checkout {{.Invalid}}"},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Field, "commands.recycle")
	assert.Contains(t, fieldErrs[0].Err.Error(), "template error")
}

func TestValidateDeep_ValidRecycleTemplate(t *testing.T) {
	cfg := validConfig(t)
	cfg.Commands = Commands{
		Recycle: []string{
			"git fetch origin",
			"git checkout {{.DefaultBranch}}",
			"git reset --hard origin/{{.DefaultBranch}}",
		},
	}

	err := cfg.ValidateDeep("")
	assert.NoError(t, err)
}

func TestValidateDeep_InvalidRulePattern(t *testing.T) {
	cfg := validConfig(t)
	cfg.Rules = []Rule{
		{Pattern: "[invalid", Commands: []string{"echo"}},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Field, "rules")
	assert.Contains(t, fieldErrs[0].Err.Error(), "invalid regex")
}

func TestValidateDeep_KeybindingBothActionAndSh(t *testing.T) {
	cfg := validConfig(t)
	cfg.Keybindings = map[string]Keybinding{
		"x": {Action: ActionRecycle, Sh: "echo test"},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Err.Error(), "cannot have both")
}

func TestValidateDeep_KeybindingNeitherActionNorSh(t *testing.T) {
	cfg := validConfig(t)
	cfg.Keybindings = map[string]Keybinding{
		"x": {Help: "does nothing"},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Err.Error(), "must have either")
}

func TestValidateDeep_KeybindingInvalidAction(t *testing.T) {
	cfg := validConfig(t)
	cfg.Keybindings = map[string]Keybinding{
		"x": {Action: "invalid"},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Err.Error(), "invalid action")
}

func TestValidateDeep_KeybindingInvalidShTemplate(t *testing.T) {
	cfg := validConfig(t)
	cfg.Keybindings = map[string]Keybinding{
		"o": {Sh: "open {{.Invalid}}"},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Err.Error(), "template error")
}

func TestValidateDeep_KeybindingValidShTemplate(t *testing.T) {
	cfg := validConfig(t)
	cfg.Keybindings = map[string]Keybinding{
		"o": {Sh: "open {{.Path}} {{.Remote}} {{.ID}} {{.Name}}"},
	}

	err := cfg.ValidateDeep("")
	assert.NoError(t, err)
}

func TestValidateDeep_GitPathNotFound(t *testing.T) {
	cfg := validConfig(t)
	cfg.GitPath = "/nonexistent/path/to/git"

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)

	hasGitError := false
	for _, e := range fieldErrs {
		if e.Field == "git_path" {
			hasGitError = true
			break
		}
	}
	assert.True(t, hasGitError, "expected error about git path")
}

func TestValidateDeep_DataDirIsFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "notadir")
	require.NoError(t, os.WriteFile(tmpFile, []byte("test"), 0o644))

	cfg := validConfig(t)
	cfg.DataDir = tmpFile

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)

	hasDataDirError := false
	for _, e := range fieldErrs {
		if e.Field == "data_dir" {
			hasDataDirError = true
			break
		}
	}
	assert.True(t, hasDataDirError, "expected error about data dir")
}

func TestValidateDeep_ConfigFileIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := validConfig(t)

	err := cfg.ValidateDeep(tmpDir)

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)

	hasConfigError := false
	for _, e := range fieldErrs {
		if e.Field == "config_file" {
			hasConfigError = true
			break
		}
	}
	assert.True(t, hasConfigError, "expected error about config file being a directory")
}

func TestWarnings_EmptyRule(t *testing.T) {
	cfg := validConfig(t)
	cfg.Rules = []Rule{
		{Pattern: ".*"},
	}

	err := cfg.ValidateDeep("")
	require.NoError(t, err)

	warnings := cfg.Warnings()
	hasWarning := false
	for _, w := range warnings {
		if w.Category == "Rules" && strings.Contains(w.Message, "neither commands nor copy") {
			hasWarning = true
			break
		}
	}
	assert.True(t, hasWarning, "expected warning about empty rule")
}

func TestWarnings_EmptyRecycleCommands(t *testing.T) {
	cfg := validConfig(t)
	cfg.Commands = Commands{
		Recycle: []string{},
	}

	err := cfg.ValidateDeep("")
	require.NoError(t, err)

	warnings := cfg.Warnings()
	hasWarning := false
	for _, w := range warnings {
		if w.Category == "Recycle Commands" {
			hasWarning = true
			break
		}
	}
	assert.True(t, hasWarning, "expected warning about empty recycle commands")
}

func TestValidateDeep_ValidRulesWithCopy(t *testing.T) {
	cfg := validConfig(t)
	cfg.Rules = []Rule{
		{Pattern: "", Copy: []string{".envrc"}},
		{Pattern: "^https://github.com/.*", Copy: []string{"*.yaml"}},
	}

	err := cfg.ValidateDeep("")
	assert.NoError(t, err)
}

func TestValidateDeep_ValidRulesWithCommandsAndCopy(t *testing.T) {
	cfg := validConfig(t)
	cfg.Rules = []Rule{
		{
			Pattern:  "^https://github.com/hay-kot/.*",
			Commands: []string{"mise trust", "task dep:sync"},
			Copy:     []string{".envrc", "configs/*.yaml"},
		},
	}

	err := cfg.ValidateDeep("")
	assert.NoError(t, err)
}

func TestGetMaxRecycled(t *testing.T) {
	intPtr := func(n int) *int { return &n }

	tests := []struct {
		name     string
		rules    []Rule
		remote   string
		expected int
	}{
		{
			name:     "default when no rules",
			rules:    nil,
			remote:   "https://github.com/foo/bar",
			expected: DefaultMaxRecycled,
		},
		{
			name: "catch-all rule sets default",
			rules: []Rule{
				{Pattern: "", MaxRecycled: intPtr(10)},
			},
			remote:   "https://github.com/foo/bar",
			expected: 10,
		},
		{
			name: "catch-all unlimited (0)",
			rules: []Rule{
				{Pattern: "", MaxRecycled: intPtr(0)},
			},
			remote:   "https://github.com/foo/bar",
			expected: 0,
		},
		{
			name: "specific rule override",
			rules: []Rule{
				{Pattern: "", MaxRecycled: intPtr(10)},
				{Pattern: "github.com/foo/.*", MaxRecycled: intPtr(2)},
			},
			remote:   "https://github.com/foo/bar",
			expected: 2,
		},
		{
			name: "specific rule unlimited override",
			rules: []Rule{
				{Pattern: "", MaxRecycled: intPtr(10)},
				{Pattern: "github.com/foo/.*", MaxRecycled: intPtr(0)},
			},
			remote:   "https://github.com/foo/bar",
			expected: 0,
		},
		{
			name: "non-matching rule falls back to catch-all",
			rules: []Rule{
				{Pattern: "", MaxRecycled: intPtr(10)},
				{Pattern: "github.com/other/.*", MaxRecycled: intPtr(2)},
			},
			remote:   "https://github.com/foo/bar",
			expected: 10,
		},
		{
			name: "last matching rule wins",
			rules: []Rule{
				{Pattern: "", MaxRecycled: intPtr(10)},
				{Pattern: "github.com/.*", MaxRecycled: intPtr(5)},
				{Pattern: "github.com/foo/.*", MaxRecycled: intPtr(2)},
			},
			remote:   "https://github.com/foo/bar",
			expected: 2,
		},
		{
			name: "rule without max_recycled inherits from previous",
			rules: []Rule{
				{Pattern: "", MaxRecycled: intPtr(10)},
				{Pattern: "github.com/foo/.*", Commands: []string{"echo test"}},
			},
			remote:   "https://github.com/foo/bar",
			expected: 10,
		},
		{
			name: "later rule with max_recycled overrides earlier without",
			rules: []Rule{
				{Pattern: "github.com/foo/.*", MaxRecycled: intPtr(3)},
				{Pattern: "github.com/foo/bar", Commands: []string{"echo"}}, // no MaxRecycled
			},
			remote:   "https://github.com/foo/bar",
			expected: 3, // inherits from earlier matching rule with MaxRecycled
		},
		{
			name: "no matching rules uses default",
			rules: []Rule{
				{Pattern: "github.com/other/.*", MaxRecycled: intPtr(2)},
			},
			remote:   "https://github.com/foo/bar",
			expected: DefaultMaxRecycled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig(t)
			cfg.Rules = tt.rules

			result := cfg.GetMaxRecycled(tt.remote)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidate_MaxRecycledNegative(t *testing.T) {
	intPtr := func(n int) *int { return &n }

	t.Run("negative in rule", func(t *testing.T) {
		cfg := validConfig(t)
		cfg.Rules = []Rule{
			{Pattern: ".*", MaxRecycled: intPtr(-5)},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_recycled")
	})

	t.Run("valid values pass", func(t *testing.T) {
		cfg := validConfig(t)
		cfg.Rules = []Rule{
			{Pattern: "", MaxRecycled: intPtr(0)}, // 0 is valid (unlimited)
			{Pattern: ".*", MaxRecycled: intPtr(5)},
		}

		err := cfg.Validate()
		assert.NoError(t, err)
	})
}
