// Package config handles configuration loading and validation for hive.
package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/hay-kot/criterio"
	"gopkg.in/yaml.v3"
)

// Built-in action names for keybindings.
const (
	ActionRecycle = "recycle"
	ActionDelete  = "delete"
)

// defaultKeybindings provides built-in keybindings that users can override.
var defaultKeybindings = map[string]Keybinding{
	"r": {
		Action:  ActionRecycle,
		Help:    "recycle",
		Confirm: "Are you sure you want to recycle this session?",
	},
	"d": {
		Action:  ActionDelete,
		Help:    "delete",
		Confirm: "Are you sure you want to delete this session?",
	},
}

// Config holds the application configuration.
type Config struct {
	Commands            Commands              `yaml:"commands"`
	Git                 GitConfig             `yaml:"git"`
	GitPath             string                `yaml:"git_path"`
	Keybindings         map[string]Keybinding `yaml:"keybindings"`
	Rules               []Rule                `yaml:"rules"`
	AutoDeleteCorrupted bool                  `yaml:"auto_delete_corrupted"`
	History             HistoryConfig         `yaml:"history"`
	Context             ContextConfig         `yaml:"context"`
	DataDir             string                `yaml:"-"` // set by caller, not from config file
}

// HistoryConfig holds command history configuration.
type HistoryConfig struct {
	MaxEntries int `yaml:"max_entries"`
}

// ContextConfig configures context directory behavior.
type ContextConfig struct {
	SymlinkName string `yaml:"symlink_name"` // default: ".hive"
}

// GitConfig holds git-related configuration.
type GitConfig struct {
	StatusWorkers int `yaml:"status_workers"`
}

// Rule defines actions to take for matching repositories.
type Rule struct {
	// Pattern matches against remote URL (regex). Empty = matches all.
	Pattern string `yaml:"pattern"`
	// Commands to run in the session directory after clone/recycle.
	Commands []string `yaml:"commands,omitempty"`
	// Copy are glob patterns to copy from source directory.
	Copy []string `yaml:"copy,omitempty"`
}

// Commands defines the shell commands used by hive.
type Commands struct {
	Spawn   []string `yaml:"spawn"`
	Recycle []string `yaml:"recycle"`
}

// Keybinding defines a TUI keybinding action.
type Keybinding struct {
	Action  string `yaml:"action"`  // built-in action name (recycle, delete)
	Help    string `yaml:"help"`    // help text shown in TUI
	Sh      string `yaml:"sh"`      // shell command template
	Confirm string `yaml:"confirm"` // confirmation prompt (empty = no confirm)
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Commands: Commands{
			Spawn: []string{},
			Recycle: []string{
				"git fetch origin",
				"git checkout {{ .DefaultBranch }}",
				"git reset --hard origin/{{ .DefaultBranch }}",
			},
		},
		Git: GitConfig{
			StatusWorkers: 3,
		},
		GitPath:             "git",
		Keybindings:         map[string]Keybinding{},
		AutoDeleteCorrupted: true,
		History: HistoryConfig{
			MaxEntries: 100,
		},
		Context: ContextConfig{
			SymlinkName: ".hive",
		},
	}
}

// Load reads configuration from the given path and sets the data directory.
// If configPath is empty or doesn't exist, returns defaults with the provided dataDir.
func Load(configPath, dataDir string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.DataDir = dataDir

	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("read config file: %w", err)
			}

			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parse config file: %w", err)
			}

			// Re-set dataDir since Unmarshal may have cleared it
			cfg.DataDir = dataDir
		}
	}

	// Merge user keybindings into defaults (user config overrides defaults)
	cfg.Keybindings = mergeKeybindings(defaultKeybindings, cfg.Keybindings)

	// Apply defaults for zero values
	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets default values for any unset configuration options.
func (c *Config) applyDefaults() {
	defaults := DefaultConfig()
	if c.Git.StatusWorkers == 0 {
		c.Git.StatusWorkers = defaults.Git.StatusWorkers
	}
	if c.History.MaxEntries == 0 {
		c.History.MaxEntries = defaults.History.MaxEntries
	}
	if c.Context.SymlinkName == "" {
		c.Context.SymlinkName = defaults.Context.SymlinkName
	}
}

// mergeKeybindings merges user keybindings into defaults.
// User keybindings override defaults for the same key.
func mergeKeybindings(defaults, user map[string]Keybinding) map[string]Keybinding {
	result := make(map[string]Keybinding, len(defaults)+len(user))

	// Copy defaults first
	maps.Copy(result, defaults)
	maps.Copy(result, user)

	return result
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	return criterio.ValidateStruct(
		criterio.Run("git_path", c.GitPath, criterio.Required[string]),
		criterio.Run("data_dir", c.DataDir, criterio.Required[string]),
		criterio.Run("git.status_workers", c.Git.StatusWorkers, criterio.Min(1)),
		c.validateKeybindingsBasic(),
	)
}

// validateKeybindingsBasic performs basic keybinding validation for the Validate() method.
func (c *Config) validateKeybindingsBasic() error {
	var errs criterio.FieldErrorsBuilder
	for key, kb := range c.Keybindings {
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
	}

	return errs.ToError()
}

// ReposDir returns the path where cloned repositories are stored.
func (c *Config) ReposDir() string {
	return filepath.Join(c.DataDir, "repos")
}

// SessionsFile returns the path to the sessions JSON file.
func (c *Config) SessionsFile() string {
	return filepath.Join(c.DataDir, "sessions.json")
}

// HistoryFile returns the path to the command history JSON file.
func (c *Config) HistoryFile() string {
	return filepath.Join(c.DataDir, "history.json")
}

// LogsDir returns the path to the logs directory.
func (c *Config) LogsDir() string {
	return filepath.Join(c.DataDir, "logs")
}

// ContextDir returns the base context directory path.
func (c *Config) ContextDir() string {
	return filepath.Join(c.DataDir, "context")
}

// RepoContextDir returns the context directory for a specific owner/repo.
func (c *Config) RepoContextDir(owner, repo string) string {
	return filepath.Join(c.ContextDir(), owner, repo)
}

// SharedContextDir returns the shared context directory.
func (c *Config) SharedContextDir() string {
	return filepath.Join(c.ContextDir(), "shared")
}

func isValidAction(action string) bool {
	switch action {
	case ActionRecycle, ActionDelete:
		return true
	default:
		return false
	}
}
