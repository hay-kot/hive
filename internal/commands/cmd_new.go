package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/hay-kot/hive/internal/core/history"
	"github.com/hay-kot/hive/internal/core/validate"
	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/hay-kot/hive/internal/styles"
	"github.com/urfave/cli/v3"
)

type NewCmd struct {
	flags *Flags

	// Command-specific flags
	name   string
	remote string
	prompt string
	replay bool
}

// NewNewCmd creates a new new command
func NewNewCmd(flags *Flags) *NewCmd {
	return &NewCmd{flags: flags}
}

// Register adds the new command to the application
func (cmd *NewCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "new",
		Usage:     "Create a new agent session",
		UsageText: "hive new [options] [replay-id]",
		Description: `Creates a new isolated git environment for an AI agent session.

If a recyclable session exists for the same remote, it will be reused
(reset, checkout main, pull). Otherwise, a fresh clone is created.

After setup, any matching hooks are executed and the configured spawn
command launches a terminal with the AI tool.

When --name is omitted, an interactive form prompts for input.

Use --replay to re-run a previous command:
  hive new --replay          # Replay the last failed 'new' command
  hive new --replay <id>     # Replay a specific command by ID`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "name",
				Aliases:     []string{"n"},
				Usage:       "session name used in the directory path",
				Destination: &cmd.name,
			},
			&cli.StringFlag{
				Name:        "remote",
				Aliases:     []string{"r"},
				Usage:       "git remote URL (defaults to current directory's origin)",
				Destination: &cmd.remote,
			},
			&cli.StringFlag{
				Name:        "prompt",
				Aliases:     []string{"p"},
				Usage:       "AI prompt passed to the spawn command template",
				Destination: &cmd.prompt,
			},
			&cli.BoolFlag{
				Name:        "replay",
				Aliases:     []string{"R"},
				Usage:       "replay a previous command (last failed, or specify ID as argument)",
				Destination: &cmd.replay,
			},
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *NewCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	// Handle replay mode
	if cmd.replay {
		return cmd.runReplay(ctx, c, p)
	}

	// Show interactive form if name not provided via flag
	if cmd.name == "" {
		if err := cmd.runForm(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return fmt.Errorf("form: %w", err)
		}
	}

	opts := hive.CreateOptions{
		Name:   cmd.name,
		Remote: cmd.remote,
		Prompt: cmd.prompt,
	}

	// Save parsed options for history recording
	cmd.flags.LastNewOptions = &history.NewOptions{
		Name:   cmd.name,
		Remote: cmd.remote,
		Prompt: cmd.prompt,
	}

	sess, err := cmd.flags.Service.CreateSession(ctx, opts)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	p.Success("Session created", sess.Path)

	return nil
}

func (cmd *NewCmd) runReplay(ctx context.Context, c *cli.Command, p *printer.Printer) error {
	entry, err := cmd.getReplayEntry(ctx, c.Args().First())
	if errors.Is(err, history.ErrNotFound) {
		p.Infof("No commands in history")
		return nil
	}
	if err != nil {
		return err
	}

	p.Infof("Replaying: hive %s", entry.CommandString())

	if entry.Options == nil {
		return fmt.Errorf("history entry %s is missing options data", entry.ID)
	}

	opts := hive.CreateOptions{
		Name:   entry.Options.Name,
		Remote: entry.Options.Remote,
		Prompt: entry.Options.Prompt,
	}

	sess, err := cmd.flags.Service.CreateSession(ctx, opts)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	p.Success("Session created", sess.Path)
	return nil
}

// getReplayEntry retrieves a history entry by ID, or the last entry if id is empty.
func (cmd *NewCmd) getReplayEntry(ctx context.Context, id string) (history.Entry, error) {
	if id == "" {
		return cmd.flags.HistoryStore.Last(ctx)
	}

	entry, err := cmd.flags.HistoryStore.Get(ctx, id)
	if errors.Is(err, history.ErrNotFound) {
		return history.Entry{}, fmt.Errorf("command %q not found in history", id)
	}
	return entry, err
}

func (cmd *NewCmd) runForm() error {
	// Print banner header
	fmt.Println(styles.BannerStyle.Render(styles.Banner))
	fmt.Println()

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Session name").
				Description("Used in the directory path").
				Validate(validate.SessionName).
				Value(&cmd.name),
			huh.NewText().
				Title("Prompt").
				Description("AI prompt to pass to spawn command").
				Value(&cmd.prompt),
		),
	).WithTheme(styles.FormTheme()).Run()
}
