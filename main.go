package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/hay-kot/hive/internal/commands"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/git"
	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/hay-kot/hive/internal/store/jsonfile"
	"github.com/hay-kot/hive/pkg/executil"
)

var (
	// Build information. Populated at build-time via -ldflags flag.
	version = "dev"
	commit  = "HEAD"
	date    = "now"
)

func build() string {
	short := commit
	if len(commit) > 7 {
		short = commit[:7]
	}

	return fmt.Sprintf("%s (%s) %s", version, short, date)
}

func setupLogger(level string, logFile string, fileOnly bool) error {
	parsedLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %w", err)
	}

	var output io.Writer = zerolog.ConsoleWriter{Out: os.Stderr}

	if logFile != "" {
		// Create log directory if it doesn't exist
		logDir := filepath.Dir(logFile)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// Open log file
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		if fileOnly {
			// Write only to file (for TUI mode)
			output = file
		} else {
			// Write to both console and file
			output = io.MultiWriter(
				zerolog.ConsoleWriter{Out: os.Stderr},
				file,
			)
		}
	} else if fileOnly {
		// TUI mode with no explicit log file - discard logs
		output = io.Discard
	}

	log.Logger = log.Output(output).Level(parsedLevel)

	return nil
}

func main() {
	if err := setupLogger("info", "", false); err != nil {
		panic(err)
	}

	p := printer.New(os.Stderr)
	ctx := printer.NewContext(context.Background(), p)

	flags := &commands.Flags{}

	app := &cli.Command{
		Name:      "hive",
		Usage:     "Manage multiple AI agent sessions",
		UsageText: "hive [global options] command [command options]",
		Description: `Hive creates isolated git environments for running multiple AI agents in parallel.

Instead of managing worktrees manually, hive handles cloning, recycling, and
spawning terminal sessions with your preferred AI tool.

Run 'hive' with no arguments to open the interactive session manager.
Run 'hive new' to create a new session from the current repository.`,
		Version: build(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "log level (debug, info, warn, error, fatal, panic)",
				Sources:     cli.EnvVars("HIVE_LOG_LEVEL"),
				Value:       "info",
				Destination: &flags.LogLevel,
			},
			&cli.StringFlag{
				Name:        "log-file",
				Usage:       "path to log file (optional)",
				Sources:     cli.EnvVars("HIVE_LOG_FILE"),
				Destination: &flags.LogFile,
			},
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Usage:       "path to config file",
				Sources:     cli.EnvVars("HIVE_CONFIG"),
				Value:       commands.DefaultConfigPath(),
				Destination: &flags.ConfigPath,
			},
			&cli.StringFlag{
				Name:        "data-dir",
				Usage:       "path to data directory",
				Sources:     cli.EnvVars("HIVE_DATA_DIR"),
				Value:       commands.DefaultDataDir(),
				Destination: &flags.DataDir,
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			// Detect TUI mode: either "tui" subcommand or no subcommand (default action)
			isTUI := len(c.Args().Slice()) == 0 || c.Args().First() == "tui"

			// In TUI mode, use file logging to avoid mangling the display
			logFile := flags.LogFile
			if isTUI && logFile == "" {
				logFile = commands.DefaultLogFile()
			}

			if err := setupLogger(flags.LogLevel, logFile, isTUI); err != nil {
				return ctx, err
			}

			cfg, err := config.Load(flags.ConfigPath, flags.DataDir)
			if err != nil {
				return ctx, fmt.Errorf("load config: %w", err)
			}
			flags.Config = cfg

			// Create service
			store := jsonfile.New(cfg.SessionsFile())
			exec := &executil.RealExecutor{}
			gitExec := git.NewExecutor(cfg.GitPath, exec)
			logger := log.With().Str("component", "hive").Logger()
			flags.Service = hive.New(store, gitExec, cfg, exec, logger, os.Stdout, os.Stderr)

			return ctx, nil
		},
	}

	tuiCmd := commands.NewTuiCmd(flags)

	app = commands.NewNewCmd(flags).Register(app)
	app = tuiCmd.Register(app)
	app = commands.NewLsCmd(flags).Register(app)
	app = commands.NewPruneCmd(flags).Register(app)

	// Set TUI as default action when no subcommand is provided
	app.Action = tuiCmd.Run

	exitCode := 0
	if err := app.Run(ctx, os.Args); err != nil {
		fmt.Println()
		printer.Ctx(ctx).FatalError(err)
		exitCode = 1
	}

	os.Exit(exitCode)
}
