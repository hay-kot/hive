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
	"github.com/hay-kot/hive/internal/printer"
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

func setupLogger(level string, logFile string) error {
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

		// Write to both console and file
		output = io.MultiWriter(
			zerolog.ConsoleWriter{Out: os.Stderr},
			file,
		)
	}

	log.Logger = log.Output(output).Level(parsedLevel)

	return nil
}

func main() {
	if err := setupLogger("info", ""); err != nil {
		panic(err)
	}

	p := printer.New(os.Stderr)
	ctx := printer.NewContext(context.Background(), p)

	flags := &commands.Flags{}

	app := &cli.Command{
		Name:    "hive",
		Usage:   `A cli/tui for managing and spinning up multiple AI agent sessions as an alternative to git worktrees`,
		Version: build(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "log level (debug, info, warn, error, fatal, panic)",
				Sources:     cli.EnvVars("LOG_LEVEL"),
				Value:       "info",
				Destination: &flags.LogLevel,
			},
			&cli.StringFlag{
				Name:        "log-file",
				Usage:       "path to log file (optional)",
				Sources:     cli.EnvVars("LOG_FILE"),
				Destination: &flags.LogFile,
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			if err := setupLogger(flags.LogLevel, flags.LogFile); err != nil {
				return ctx, err
			}

			return ctx, nil
		},
	}
	app = commands.NewNewCmd(flags).Register(app)
	app = commands.NewTuiCmd(flags).Register(app)
	app = commands.NewLsCmd(flags).Register(app)

	exitCode := 0
	if err := app.Run(ctx, os.Args); err != nil {
		fmt.Println()
		printer.Ctx(ctx).FatalError(err)
		exitCode = 1
	}

	os.Exit(exitCode)
}
