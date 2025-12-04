package config

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"text/template"
)

// ValidationResult holds the outcome of configuration validation.
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationWarning
	Checks   []ValidationCheck
}

// ValidationError represents a configuration error.
type ValidationError struct {
	Category string
	Item     string
	Message  string
	Fix      string
}

// ValidationWarning represents a non-fatal configuration issue.
type ValidationWarning struct {
	Category string
	Item     string
	Message  string
}

// ValidationCheck represents a successful validation check.
type ValidationCheck struct {
	Category string
	Message  string
	Details  []string
}

// IsValid returns true if there are no errors.
func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// ErrorCount returns the number of errors.
func (r *ValidationResult) ErrorCount() int {
	return len(r.Errors)
}

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

// ValidateDeep performs comprehensive validation of the configuration.
// Unlike Validate(), this checks template syntax, regex patterns, and file access.
func (c *Config) ValidateDeep(configPath string) *ValidationResult {
	result := &ValidationResult{}

	c.validateFileAccess(result, configPath)
	c.validateSpawnCommands(result)
	c.validateRecycleCommands(result)
	c.validateHooks(result)
	c.validateKeybindings(result)

	return result
}

// validateFileAccess checks config file, data directory, and git executable.
func (c *Config) validateFileAccess(result *ValidationResult, configPath string) {
	details := []string{}

	// Check config file
	if configPath != "" {
		if info, err := os.Stat(configPath); err == nil {
			details = append(details, fmt.Sprintf("Config file: %s (found)", configPath))
			if info.IsDir() {
				result.Errors = append(result.Errors, ValidationError{
					Category: "File Access",
					Item:     "config file",
					Message:  fmt.Sprintf("%s is a directory, not a file", configPath),
				})
			}
		} else if os.IsNotExist(err) {
			details = append(details, fmt.Sprintf("Config file: %s (not found, using defaults)", configPath))
		} else {
			result.Errors = append(result.Errors, ValidationError{
				Category: "File Access",
				Item:     "config file",
				Message:  fmt.Sprintf("cannot access %s: %v", configPath, err),
			})
		}
	}

	// Check git path
	if c.GitPath != "" {
		gitPath, err := exec.LookPath(c.GitPath)
		if err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Category: "File Access",
				Item:     "git_path",
				Message:  fmt.Sprintf("git executable not found: %s", c.GitPath),
				Fix:      "Set git_path to the full path of your git executable",
			})
		} else {
			details = append(details, fmt.Sprintf("Git path: %s (found)", gitPath))
		}
	}

	// Check data directory
	if c.DataDir != "" {
		if info, err := os.Stat(c.DataDir); err == nil {
			if !info.IsDir() {
				result.Errors = append(result.Errors, ValidationError{
					Category: "File Access",
					Item:     "data_dir",
					Message:  fmt.Sprintf("%s exists but is not a directory", c.DataDir),
				})
			} else {
				details = append(details, fmt.Sprintf("Data directory: %s (exists)", c.DataDir))
			}
		} else if os.IsNotExist(err) {
			details = append(details, fmt.Sprintf("Data directory: %s (will be created)", c.DataDir))
		} else {
			result.Errors = append(result.Errors, ValidationError{
				Category: "File Access",
				Item:     "data_dir",
				Message:  fmt.Sprintf("cannot access %s: %v", c.DataDir, err),
			})
		}
	}

	if len(details) > 0 {
		result.Checks = append(result.Checks, ValidationCheck{
			Category: "File Access",
			Message:  "File paths validated",
			Details:  details,
		})
	}
}

// validateSpawnCommands checks template syntax for spawn commands.
func (c *Config) validateSpawnCommands(result *ValidationResult) {
	if len(c.Commands.Spawn) == 0 {
		result.Checks = append(result.Checks, ValidationCheck{
			Category: "Spawn Commands",
			Message:  "No spawn commands defined",
		})
		return
	}

	details := []string{}
	for i, cmd := range c.Commands.Spawn {
		if err := validateTemplate(cmd, SpawnTemplateData{}); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Category: "Spawn Commands",
				Item:     fmt.Sprintf("command %d", i),
				Message:  fmt.Sprintf("template error: %v", err),
				Fix:      "Check template syntax. Available variables: {{.Path}}, {{.Name}}, {{.Prompt}}",
			})
		} else {
			details = append(details, fmt.Sprintf("Command %d: valid template", i))
		}
	}

	if len(details) > 0 {
		result.Checks = append(result.Checks, ValidationCheck{
			Category: "Spawn Commands",
			Message:  fmt.Sprintf("%d command(s) defined", len(c.Commands.Spawn)),
			Details:  details,
		})
	}
}

// validateRecycleCommands checks recycle commands (no templating, just presence).
func (c *Config) validateRecycleCommands(result *ValidationResult) {
	if len(c.Commands.Recycle) == 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Category: "Recycle Commands",
			Item:     "commands",
			Message:  "No recycle commands defined; sessions will only be marked as recycled",
		})
		return
	}

	details := make([]string, len(c.Commands.Recycle))
	for i, cmd := range c.Commands.Recycle {
		details[i] = fmt.Sprintf("Command %d: %s", i, cmd)
	}

	result.Checks = append(result.Checks, ValidationCheck{
		Category: "Recycle Commands",
		Message:  fmt.Sprintf("%d command(s) defined", len(c.Commands.Recycle)),
		Details:  details,
	})
}

// validateHooks checks hook patterns are valid regex.
func (c *Config) validateHooks(result *ValidationResult) {
	if len(c.Hooks) == 0 {
		result.Checks = append(result.Checks, ValidationCheck{
			Category: "Hooks",
			Message:  "No hooks defined",
		})
		return
	}

	details := []string{}
	for i, hook := range c.Hooks {
		// Check pattern is valid regex
		if _, err := regexp.Compile(hook.Pattern); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Category: "Hooks",
				Item:     fmt.Sprintf("pattern %d", i),
				Message:  fmt.Sprintf("invalid regex %q: %v", hook.Pattern, err),
				Fix:      "Use valid regex syntax. Note: hive uses regex, not glob patterns",
			})

			// Check if it looks like a glob pattern
			if looksLikeGlob(hook.Pattern) {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Category: "Hooks",
					Item:     fmt.Sprintf("pattern %d", i),
					Message:  fmt.Sprintf("pattern %q looks like a glob pattern; use regex instead", hook.Pattern),
				})
			}
		} else {
			details = append(details, fmt.Sprintf("Pattern %d: %s (valid regex)", i, hook.Pattern))
		}

		// Check commands are present
		if len(hook.Commands) == 0 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Category: "Hooks",
				Item:     fmt.Sprintf("hook %d", i),
				Message:  "hook has no commands defined",
			})
		}
	}

	if len(details) > 0 {
		result.Checks = append(result.Checks, ValidationCheck{
			Category: "Hooks",
			Message:  fmt.Sprintf("%d hook(s) defined", len(c.Hooks)),
			Details:  details,
		})
	}
}

// validateKeybindings checks keybinding configuration.
func (c *Config) validateKeybindings(result *ValidationResult) {
	if len(c.Keybindings) == 0 {
		result.Checks = append(result.Checks, ValidationCheck{
			Category: "Keybindings",
			Message:  "No keybindings defined (using defaults)",
		})
		return
	}

	details := []string{}
	for key, kb := range c.Keybindings {
		// Check action vs shell
		if kb.Action == "" && kb.Sh == "" {
			result.Errors = append(result.Errors, ValidationError{
				Category: "Keybindings",
				Item:     fmt.Sprintf("key %q", key),
				Message:  "must have either action or sh",
				Fix:      "Add either 'action: recycle' or 'action: delete' or a 'sh: <command>' field",
			})
			continue
		}

		if kb.Action != "" && kb.Sh != "" {
			result.Errors = append(result.Errors, ValidationError{
				Category: "Keybindings",
				Item:     fmt.Sprintf("key %q", key),
				Message:  "cannot have both action and sh",
				Fix:      "Remove either the 'action' or 'sh' field",
			})
			continue
		}

		// Validate action type
		if kb.Action != "" {
			if !isValidAction(kb.Action) {
				result.Errors = append(result.Errors, ValidationError{
					Category: "Keybindings",
					Item:     fmt.Sprintf("key %q", key),
					Message:  fmt.Sprintf("invalid action %q", kb.Action),
					Fix:      "Use 'recycle' or 'delete'",
				})
			} else {
				help := kb.Help
				if help == "" {
					help = kb.Action
				}
				details = append(details, fmt.Sprintf("%s: %s (valid)", key, help))
			}
		}

		// Validate shell template
		if kb.Sh != "" {
			if err := validateTemplate(kb.Sh, KeybindingTemplateData{}); err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Category: "Keybindings",
					Item:     fmt.Sprintf("key %q", key),
					Message:  fmt.Sprintf("template error in sh: %v", err),
					Fix:      "Check template syntax. Available variables: {{.Path}}, {{.Remote}}, {{.ID}}, {{.Name}}",
				})
			} else {
				help := kb.Help
				if help == "" {
					help = "shell command"
				}
				details = append(details, fmt.Sprintf("%s: %s (valid template)", key, help))
			}
		}
	}

	if len(details) > 0 {
		result.Checks = append(result.Checks, ValidationCheck{
			Category: "Keybindings",
			Message:  fmt.Sprintf("%d keybinding(s) defined", len(c.Keybindings)),
			Details:  details,
		})
	}
}

// validateTemplate checks if a template string is valid.
func validateTemplate(tmplStr string, data any) error {
	t, err := template.New("").Option("missingkey=error").Parse(tmplStr)
	if err != nil {
		return err
	}

	// Dry-run execute to catch missing key errors
	// We pass empty/zero data so missing keys are caught
	var buf struct{}
	_ = buf
	return t.Execute(&nopWriter{}, data)
}

// nopWriter discards all writes.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// looksLikeGlob checks if a pattern appears to be a glob rather than regex.
func looksLikeGlob(pattern string) bool {
	// Glob patterns often use ** or *. without proper regex escaping
	globIndicators := []string{"**", "*.", "*."}
	for _, indicator := range globIndicators {
		if contains(pattern, indicator) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
