package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hay-kot/criterio"
	"github.com/hay-kot/hive/internal/core/validate"
	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/pkg/randid"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

const (
	// StatusCreated indicates the session was created successfully.
	StatusCreated = "created"
	// StatusFailed indicates the session creation failed.
	StatusFailed = "failed"
	// StatusSkipped indicates the session was not attempted due to failure threshold.
	StatusSkipped = "skipped"

	// maxFailures is the number of failures before stopping batch processing.
	maxFailures = 3
)

// BatchInput is the JSON input schema for batch session creation.
type BatchInput struct {
	Sessions []BatchSession `json:"sessions"`
}

// Validate checks the batch input for errors using criterio.
func (b BatchInput) Validate() error {
	if len(b.Sessions) == 0 {
		return criterio.NewFieldErrors("sessions", fmt.Errorf("array is empty"))
	}

	var errs criterio.FieldErrorsBuilder
	seenNames := make(map[string]bool)
	seenIDs := make(map[string]bool)

	for i, sess := range b.Sessions {
		field := fmt.Sprintf("sessions[%d]", i)

		if err := validate.SessionName(sess.Name); err != nil {
			errs = errs.Append(field+".name", err)
			continue
		}

		if seenNames[sess.Name] {
			errs = errs.Append(field+".name", fmt.Errorf("duplicate name %q", sess.Name))
			continue
		}
		seenNames[sess.Name] = true

		// Validate session_id if provided
		if sess.SessionID != "" {
			if err := validate.SessionID(sess.SessionID); err != nil {
				errs = errs.Append(field+".session_id", err)
				continue
			}
			if seenIDs[sess.SessionID] {
				errs = errs.Append(field+".session_id", fmt.Errorf("duplicate session_id %q", sess.SessionID))
				continue
			}
			seenIDs[sess.SessionID] = true
		}
	}

	return errs.ToError()
}

// BatchSession defines a single session to create.
type BatchSession struct {
	Name      string `json:"name"`
	SessionID string `json:"session_id,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Remote    string `json:"remote,omitempty"`
	Source    string `json:"source,omitempty"`
}

// BatchResult is the output for a single session creation attempt.
type BatchResult struct {
	Name      string `json:"name"`
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// BatchOutput is the JSON output schema.
type BatchOutput struct {
	BatchID string        `json:"batch_id"`
	LogFile string        `json:"log_file"`
	Results []BatchResult `json:"results"`
}

// BatchErrorOutput is the JSON output for fatal errors.
type BatchErrorOutput struct {
	Error string `json:"error"`
}

type BatchCmd struct {
	flags *Flags
	file  string
}

func NewBatchCmd(flags *Flags) *BatchCmd {
	return &BatchCmd{flags: flags}
}

func (cmd *BatchCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "batch",
		Usage: "Create multiple sessions from JSON input",
		UsageText: `hive batch [options]

Read from stdin:
  echo '{"sessions":[{"name":"task1","prompt":"Do something"}]}' | hive batch

Read from file:
  hive batch -f sessions.json`,
		Description: `Creates multiple agent sessions from a JSON specification.

Each session in the input array is created sequentially. A terminal is
spawned for each session using the batch_spawn commands if configured,
otherwise falls back to spawn commands.

Processing stops after 3 failures. Sessions not attempted are marked as skipped.

Input JSON schema:
  {
    "sessions": [
      {
        "name": "session-name",
        "session_id": "optional-id",
        "prompt": "optional task prompt",
        "remote": "optional-url",
        "source": "optional-path"
      }
    ]
  }

Fields:
  name       - Required. Session name (used in path).
  session_id - Optional. Session ID (lowercase alphanumeric, auto-generated if empty).
  prompt     - Optional. Task prompt passed to batch_spawn via {{.Prompt}} template.
  remote     - Optional. Git remote URL (auto-detected from current dir if empty).
  source     - Optional. Directory to copy files from (per copy rules in config).

Config example (in ~/.config/hive/config.yaml):
  commands:
    spawn:        # Used by hive new
      - "wezterm cli spawn --cwd {{.Path}}"
    batch_spawn:  # Used by hive batch (supports {{.Prompt}})
      - "wezterm cli spawn --cwd {{.Path}} -- claude --prompt '{{.Prompt}}'"

Output is JSON with a batch ID, log file path, and results for each session.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "file",
				Aliases:     []string{"f"},
				Usage:       "path to JSON file (reads from stdin if not provided)",
				Destination: &cmd.file,
			},
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *BatchCmd) run(ctx context.Context, c *cli.Command) error {
	batchID := randid.Generate(6)

	logger, logFile, err := cmd.setupLogger(batchID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "batch %s: failed to setup logger: %v\n", batchID, err)
		return cmd.writeError(fmt.Errorf("setup logger: %w", err))
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close log file: %v\n", err)
		}
	}()

	logger.Info().Str("batch_id", batchID).Msg("starting batch processing")

	input, err := cmd.readInput()
	if err != nil {
		logger.Error().Err(err).Msg("failed to read input")
		return cmd.writeError(fmt.Errorf("read input: %w", err))
	}

	if err := input.Validate(); err != nil {
		logger.Error().Err(err).Msg("input validation failed")
		return cmd.writeError(fmt.Errorf("invalid input: %w", err))
	}

	output := BatchOutput{
		BatchID: batchID,
		LogFile: filepath.Join(cmd.flags.Config.LogsDir(), fmt.Sprintf("batch-%s.log", batchID)),
		Results: make([]BatchResult, 0, len(input.Sessions)),
	}

	failures := 0
	for i, sess := range input.Sessions {
		if failures >= maxFailures {
			logger.Warn().Str("name", sess.Name).Msg("skipping session due to failure threshold")
			for j := i; j < len(input.Sessions); j++ {
				output.Results = append(output.Results, BatchResult{
					Name:   input.Sessions[j].Name,
					Status: StatusSkipped,
				})
			}
			break
		}

		logger.Info().Str("name", sess.Name).Int("index", i).Msg("creating session")

		result := cmd.createSession(ctx, sess)
		output.Results = append(output.Results, result)

		if result.Status == StatusFailed {
			failures++
			logger.Error().Str("name", sess.Name).Str("error", result.Error).Msg("session creation failed")
		} else {
			logger.Info().Str("name", sess.Name).Str("session_id", result.SessionID).Msg("session created")
		}
	}

	logger.Info().
		Int("total", len(input.Sessions)).
		Int("created", countByStatus(output.Results, StatusCreated)).
		Int("failed", countByStatus(output.Results, StatusFailed)).
		Int("skipped", countByStatus(output.Results, StatusSkipped)).
		Msg("batch processing complete")

	return cmd.writeOutput(output)
}

func (cmd *BatchCmd) setupLogger(batchID string) (zerolog.Logger, *os.File, error) {
	logsDir := cmd.flags.Config.LogsDir()
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("create logs dir: %w", err)
	}

	logPath := filepath.Join(logsDir, fmt.Sprintf("batch-%s.log", batchID))
	file, err := os.Create(logPath)
	if err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("create log file: %w", err)
	}

	logger := zerolog.New(file).With().Timestamp().Logger()
	return logger, file, nil
}

func (cmd *BatchCmd) readInput() (BatchInput, error) {
	var reader io.Reader

	if cmd.file != "" {
		f, err := os.Open(cmd.file)
		if err != nil {
			return BatchInput{}, fmt.Errorf("open file: %w", err)
		}
		defer func() { _ = f.Close() }()
		reader = f
	} else {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			return BatchInput{}, fmt.Errorf("no input provided (stdin is a terminal); use -f flag or pipe JSON input")
		}
		reader = os.Stdin
	}

	var input BatchInput
	if err := json.NewDecoder(reader).Decode(&input); err != nil {
		return BatchInput{}, fmt.Errorf("decode JSON: %w", err)
	}

	return input, nil
}

func (cmd *BatchCmd) createSession(ctx context.Context, sess BatchSession) BatchResult {
	source := sess.Source
	if source == "" {
		var err error
		source, err = os.Getwd()
		if err != nil {
			return BatchResult{
				Name:   sess.Name,
				Status: StatusFailed,
				Error:  fmt.Errorf("determine source directory: %w", err).Error(),
			}
		}
	}

	opts := hive.CreateOptions{
		Name:          sess.Name,
		SessionID:     sess.SessionID,
		Prompt:        sess.Prompt,
		Remote:        sess.Remote,
		Source:        source,
		UseBatchSpawn: true,
	}

	created, err := cmd.flags.Service.CreateSession(ctx, opts)
	if err != nil {
		return BatchResult{
			Name:   sess.Name,
			Status: StatusFailed,
			Error:  err.Error(),
		}
	}

	return BatchResult{
		Name:      sess.Name,
		SessionID: created.ID,
		Path:      created.Path,
		Status:    StatusCreated,
	}
}

func (cmd *BatchCmd) writeOutput(output BatchOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write JSON output: %v\n", err)
		fmt.Fprintf(os.Stderr, "batch_id: %s\n", output.BatchID)
		fmt.Fprintf(os.Stderr, "log_file: %s\n", output.LogFile)
		fmt.Fprintf(os.Stderr, "results: %d created, %d failed, %d skipped\n",
			countByStatus(output.Results, StatusCreated),
			countByStatus(output.Results, StatusFailed),
			countByStatus(output.Results, StatusSkipped))
		return err
	}
	return nil
}

func (cmd *BatchCmd) writeError(err error) error {
	output := BatchErrorOutput{Error: err.Error()}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(output); encErr != nil {
		fmt.Fprintf(os.Stderr, "error: %s (failed to write JSON: %v)\n", err, encErr)
	}
	return err
}

func countByStatus(results []BatchResult, status string) int {
	count := 0
	for _, r := range results {
		if r.Status == status {
			count++
		}
	}
	return count
}
