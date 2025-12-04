package commands

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/hive"
)

type Flags struct {
	LogLevel   string
	LogFile    string
	ConfigPath string
	DataDir    string

	// Config is loaded in the Before hook and available to all commands
	Config *config.Config

	// Service is the hive service for orchestrating operations
	Service *hive.Service
}

// DefaultConfigPath returns the default config file path using XDG_CONFIG_HOME.
func DefaultConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, _ := os.UserHomeDir()
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "hive", "config.yaml")
}

// DefaultDataDir returns the default data directory using XDG_DATA_HOME.
func DefaultDataDir() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "hive")
}

// DefaultLogFile returns the default log file path using the system's state directory.
// On macOS: ~/Library/Logs/hive/hive.log
// On Linux: $XDG_STATE_HOME/hive/hive.log (defaults to ~/.local/state/hive/hive.log)
func DefaultLogFile() string {
	// Check XDG_STATE_HOME first (works on both macOS and Linux)
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome != "" {
		return filepath.Join(stateHome, "hive", "hive.log")
	}

	home, _ := os.UserHomeDir()

	// On macOS, use ~/Library/Logs
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Logs", "hive", "hive.log")
	}

	// On Linux, use ~/.local/state
	return filepath.Join(home, ".local", "state", "hive", "hive.log")
}
