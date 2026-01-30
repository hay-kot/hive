package commands

import (
	"os"
	"path/filepath"

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
