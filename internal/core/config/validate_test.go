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

	err := cfg.ValidateDeep("")
	assert.NoError(t, err, "expected valid config")
}

func TestValidateDeep_InvalidSpawnTemplate(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Commands: Commands{
			Spawn: []string{"echo {{.Path}", "echo {{.Invalid}}"},
		},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 2)
	assert.Contains(t, fieldErrs[0].Field, "commands.spawn")
	assert.Contains(t, fieldErrs[0].Err.Error(), "template error")
}

func TestValidateDeep_InvalidHookPattern(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Hooks: []Hook{
			{Pattern: "[invalid", Commands: []string{"echo"}},
		},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Field, "hooks")
	assert.Contains(t, fieldErrs[0].Err.Error(), "invalid regex")
}

func TestValidateDeep_KeybindingBothActionAndSh(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"x": {Action: ActionRecycle, Sh: "echo test"},
		},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Err.Error(), "cannot have both")
}

func TestValidateDeep_KeybindingNeitherActionNorSh(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"x": {Help: "does nothing"},
		},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Err.Error(), "must have either")
}

func TestValidateDeep_KeybindingInvalidAction(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"x": {Action: "invalid"},
		},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Err.Error(), "invalid action")
}

func TestValidateDeep_KeybindingInvalidShTemplate(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"o": {Sh: "open {{.Invalid}}"},
		},
	}

	err := cfg.ValidateDeep("")

	var fieldErrs criterio.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 1)
	assert.Contains(t, fieldErrs[0].Err.Error(), "template error")
}

func TestValidateDeep_KeybindingValidShTemplate(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Keybindings: map[string]Keybinding{
			"o": {Sh: "open {{.Path}} {{.Remote}} {{.ID}} {{.Name}}"},
		},
	}

	err := cfg.ValidateDeep("")
	assert.NoError(t, err)
}

func TestValidateDeep_GitPathNotFound(t *testing.T) {
	cfg := &Config{
		GitPath: "/nonexistent/path/to/git",
		DataDir: t.TempDir(),
	}

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

	cfg := &Config{
		GitPath: "git",
		DataDir: tmpFile,
	}

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

	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
	}

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

func TestWarnings_EmptyHookCommands(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Hooks: []Hook{
			{Pattern: ".*", Commands: []string{}},
		},
	}

	// Should be valid
	err := cfg.ValidateDeep("")
	require.NoError(t, err)

	// But should have a warning
	warnings := cfg.Warnings()
	hasWarning := false
	for _, w := range warnings {
		if w.Category == "Hooks" && strings.Contains(w.Message, "no commands") {
			hasWarning = true
			break
		}
	}
	assert.True(t, hasWarning, "expected warning about empty hook commands")
}

func TestWarnings_EmptyRecycleCommands(t *testing.T) {
	cfg := &Config{
		GitPath: "git",
		DataDir: t.TempDir(),
		Commands: Commands{
			Recycle: []string{},
		},
	}

	// Should be valid
	err := cfg.ValidateDeep("")
	require.NoError(t, err)

	// But should have a warning
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
