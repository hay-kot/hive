// Package config handles configuration loading and validation for hive.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Built-in action names for keybindings.
const (
	ActionRecycle = "recycle"
	ActionDelete  = "delete"
)

// Config holds the application configuration.
type Config struct {
	Commands    Commands              `yaml:"commands"`
	GitPath     string                `yaml:"git_path"`
	Keybindings map[string]Keybinding `yaml:"keybindings"`
	DataDir     string                `yaml:"-"` // set by caller, not from config file
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
			Spawn:   []string{},
			Recycle: []string{"git reset --hard", "git checkout main", "git pull"},
		},
		GitPath:     "git",
		Keybindings: map[string]Keybinding{},
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.GitPath == "" {
		return fmt.Errorf("git_path cannot be empty")
	}

	if c.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	for key, kb := range c.Keybindings {
		if kb.Action == "" && kb.Sh == "" {
			return fmt.Errorf("keybinding %q must have either action or sh", key)
		}
		if kb.Action != "" && kb.Sh != "" {
			return fmt.Errorf("keybinding %q cannot have both action and sh", key)
		}
		if kb.Action != "" {
			if !isValidAction(kb.Action) {
				return fmt.Errorf("keybinding %q has invalid action %q", key, kb.Action)
			}
		}
	}

	return nil
}

// ReposDir returns the path where cloned repositories are stored.
func (c *Config) ReposDir() string {
	return filepath.Join(c.DataDir, "repos")
}

// SessionsFile returns the path to the sessions JSON file.
func (c *Config) SessionsFile() string {
	return filepath.Join(c.DataDir, "sessions.json")
}

func isValidAction(action string) bool {
	switch action {
	case ActionRecycle, ActionDelete:
		return true
	default:
		return false
	}
}
