package config

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/hay-kot/criterio"
	"github.com/hay-kot/hive/pkg/tmpl"
)

// SpawnTemplateData defines available fields for spawn command templates.
type SpawnTemplateData struct {
	Path       string // Absolute path to the session directory
	Name       string // Session name (directory basename)
	Prompt     string // User-provided prompt from spawn command
	Slug       string // Session slug (URL-safe version of name)
	ContextDir string // Path to context directory
	Owner      string // Repository owner
	Repo       string // Repository name
}

// RecycleTemplateData defines available fields for recycle command templates.
type RecycleTemplateData struct {
	DefaultBranch string // Default branch name (e.g., "main" or "master")
}

// KeybindingTemplateData defines available fields for keybinding shell templates.
type KeybindingTemplateData struct {
	Path   string // Absolute path to the session directory
	Remote string // Git remote URL (origin)
	ID     string // Unique session identifier
	Name   string // Session name (directory basename)
}

// ValidationWarning represents a non-fatal configuration issue.
type ValidationWarning struct {
	Category string `json:"category"`
	Item     string `json:"item,omitempty"`
	Message  string `json:"message"`
}

// ValidateDeep performs comprehensive validation of the configuration including
// template syntax, regex patterns, and file accessibility. The configPath argument
// specifies the config file location to validate (empty string skips config file check).
// This calls Validate() first for basic structural validation, then adds I/O checks.
func (c *Config) ValidateDeep(configPath string) error {
	if err := c.Validate(); err != nil {
		return err
	}

	return criterio.ValidateStruct(
		c.validateFileAccess(configPath),
		validateTemplates("commands.spawn", c.Commands.Spawn, SpawnTemplateData{}),
		validateTemplates("commands.recycle", c.Commands.Recycle, RecycleTemplateData{}),
		c.validateRules(),
		c.validateKeybindingTemplates(),
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

	for i, rule := range c.Rules {
		if len(rule.Commands) == 0 && len(rule.Copy) == 0 {
			warnings = append(warnings, ValidationWarning{
				Category: "Rules",
				Item:     fmt.Sprintf("rule %d", i),
				Message:  "rule has neither commands nor copy defined",
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

// validateTemplates checks template syntax for a slice of command templates.
func validateTemplates(fieldPrefix string, commands []string, data any) error {
	var errs criterio.FieldErrorsBuilder
	for i, cmd := range commands {
		if err := validateTemplate(cmd, data); err != nil {
			errs = errs.Append(fmt.Sprintf("%s[%d]", fieldPrefix, i), fmt.Errorf("template error: %w", err))
		}
	}
	return errs.ToError()
}

// validateRules checks rule patterns are valid regex.
func (c *Config) validateRules() error {
	var errs criterio.FieldErrorsBuilder
	for i, rule := range c.Rules {
		if rule.Pattern == "" {
			continue // empty pattern matches all, valid
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			errs = errs.Append(fmt.Sprintf("rules[%d].pattern", i), fmt.Errorf("invalid regex %q: %w", rule.Pattern, err))
		}
	}
	return errs.ToError()
}

// validateKeybindingTemplates checks template syntax for keybinding shell commands.
// Basic keybinding structure validation is done by Validate().
func (c *Config) validateKeybindingTemplates() error {
	var errs criterio.FieldErrorsBuilder
	for key, kb := range c.Keybindings {
		if kb.Sh != "" {
			if err := validateTemplate(kb.Sh, KeybindingTemplateData{}); err != nil {
				errs = errs.Append(fmt.Sprintf("keybindings[%q]", key), fmt.Errorf("template error in sh: %w", err))
			}
		}
	}
	return errs.ToError()
}

// validateTemplate checks if a template string is valid.
func validateTemplate(tmplStr string, data any) error {
	_, err := tmpl.Render(tmplStr, data)
	return err
}
