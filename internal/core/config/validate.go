package config

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"text/template"

	"github.com/hay-kot/criterio"
)

// SpawnTemplateData defines available fields for spawn command templates.
type SpawnTemplateData struct {
	Path   string
	Name   string
	Prompt string
}

// KeybindingTemplateData defines available fields for keybinding shell templates.
type KeybindingTemplateData struct {
	Path   string
	Remote string
	ID     string
	Name   string
}

// ValidationWarning represents a non-fatal configuration issue.
type ValidationWarning struct {
	Category string `json:"category"`
	Item     string `json:"item,omitempty"`
	Message  string `json:"message"`
}

// ValidateDeep performs comprehensive validation of the configuration.
// Unlike Validate(), this checks template syntax, regex patterns, and file access.
func (c *Config) ValidateDeep(configPath string) error {
	return criterio.ValidateStruct(
		c.validateFileAccess(configPath),
		c.validateSpawnCommands(),
		c.validateHooks(),
		c.validateKeybindings(),
	)
}

// Warnings returns non-fatal configuration issues.
func (c *Config) Warnings() []ValidationWarning {
	var warnings []ValidationWarning

	if len(c.Commands.Recycle) == 0 {
		warnings = append(warnings, ValidationWarning{
			Category: "Recycle Commands",
			Item:     "commands",
			Message:  "No recycle commands defined; sessions will only be marked as recycled",
		})
	}

	for i, hook := range c.Hooks {
		if len(hook.Commands) == 0 {
			warnings = append(warnings, ValidationWarning{
				Category: "Hooks",
				Item:     fmt.Sprintf("hook %d", i),
				Message:  "hook has no commands defined",
			})
		}
	}

	return warnings
}

// validateFileAccess checks config file, data directory, and git executable.
func (c *Config) validateFileAccess(configPath string) error {
	return criterio.ValidateStruct(
		validateConfigFile(configPath),
		criterio.Run("git_path", c.GitPath, gitExecutableExists),
		criterio.Run("data_dir", c.DataDir, isDirectoryOrNotExist),
	)
}

func validateConfigFile(configPath string) error {
	if configPath == "" {
		return nil
	}

	info, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		return nil // not found is fine, using defaults
	}
	if err != nil {
		return criterio.NewFieldErrors("config_file", fmt.Errorf("cannot access: %w", err))
	}
	if info.IsDir() {
		return criterio.NewFieldErrors("config_file", fmt.Errorf("%s is a directory, not a file", configPath))
	}
	return nil
}

// gitExecutableExists validates that the git path is executable.
func gitExecutableExists(path string) error {
	if path == "" {
		return nil
	}
	if _, err := exec.LookPath(path); err != nil {
		return fmt.Errorf("executable not found: %s", path)
	}
	return nil
}

// isDirectoryOrNotExist validates that a path is a directory or doesn't exist.
func isDirectoryOrNotExist(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil // will be created
	}
	if err != nil {
		return fmt.Errorf("cannot access: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("exists but is not a directory")
	}
	return nil
}

// validateSpawnCommands checks template syntax for spawn commands.
func (c *Config) validateSpawnCommands() error {
	var errs criterio.FieldErrorsBuilder
	for i, cmd := range c.Commands.Spawn {
		if err := validateTemplate(cmd, SpawnTemplateData{}); err != nil {
			errs = errs.Append(fmt.Sprintf("commands.spawn[%d]", i), fmt.Errorf("template error: %w", err))
		}
	}
	return errs.ToError()
}

// validateHooks checks hook patterns are valid regex.
func (c *Config) validateHooks() error {
	var errs criterio.FieldErrorsBuilder
	for i, hook := range c.Hooks {
		if _, err := regexp.Compile(hook.Pattern); err != nil {
			errs = errs.Append(fmt.Sprintf("hooks[%d].pattern", i), fmt.Errorf("invalid regex %q: %w", hook.Pattern, err))
		}
	}
	return errs.ToError()
}

// validateKeybindings checks keybinding configuration.
func (c *Config) validateKeybindings() error {
	if len(c.Keybindings) == 0 {
		return nil
	}

	keys := make([]string, 0, len(c.Keybindings))
	for k := range c.Keybindings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var errs criterio.FieldErrorsBuilder
	for _, key := range keys {
		kb := c.Keybindings[key]
		field := fmt.Sprintf("keybindings[%q]", key)

		if kb.Action == "" && kb.Sh == "" {
			errs = errs.Append(field, fmt.Errorf("must have either action or sh"))
			continue
		}

		if kb.Action != "" && kb.Sh != "" {
			errs = errs.Append(field, fmt.Errorf("cannot have both action and sh"))
			continue
		}

		if kb.Action != "" && !isValidAction(kb.Action) {
			errs = errs.Append(field, fmt.Errorf("invalid action %q", kb.Action))
		}

		if kb.Sh != "" {
			if err := validateTemplate(kb.Sh, KeybindingTemplateData{}); err != nil {
				errs = errs.Append(field, fmt.Errorf("template error in sh: %w", err))
			}
		}
	}

	return errs.ToError()
}

// validateTemplate checks if a template string is valid.
func validateTemplate(tmplStr string, data any) error {
	t, err := template.New("").Option("missingkey=error").Parse(tmplStr)
	if err != nil {
		return err
	}

	// Dry-run execute to catch missing key errors
	return t.Execute(io.Discard, data)
}
