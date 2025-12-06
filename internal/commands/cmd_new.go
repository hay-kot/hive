package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
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
		UsageText: "hive new [options]",
		Description: `Creates a new isolated git environment for an AI agent session.

If a recyclable session exists for the same remote, it will be reused
(reset, checkout main, pull). Otherwise, a fresh clone is created.

After setup, any matching hooks are executed and the configured spawn
command launches a terminal with the AI tool.

When --name is omitted, an interactive form prompts for input.`,
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
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *NewCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

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

	sess, err := cmd.flags.Service.CreateSession(ctx, opts)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	p.Success("Session created", sess.Path)

	return nil
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
				Validate(validateName).
				Value(&cmd.name),
			huh.NewText().
				Title("Prompt").
				Description("AI prompt to pass to spawn command").
				Value(&cmd.prompt),
		),
	).WithTheme(styles.FormTheme()).Run()
}

func validateName(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
