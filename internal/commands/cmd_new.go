package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

// NewCmd implements the new command
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
		Name:  "new",
		Usage: "Create a new session",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "name",
				Aliases:     []string{"n"},
				Usage:       "Session name (used in directory path)",
				Destination: &cmd.name,
			},
			&cli.StringFlag{
				Name:        "remote",
				Aliases:     []string{"r"},
				Usage:       "Git remote URL (auto-detected from current directory if not specified)",
				Destination: &cmd.remote,
			},
			&cli.StringFlag{
				Name:        "prompt",
				Aliases:     []string{"p"},
				Usage:       "AI prompt to pass to spawn command",
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
			if err == huh.ErrUserAborted {
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

	p.Successf("Created session %s at %s", sess.ID, sess.Path)

	return nil
}

func (cmd *NewCmd) runForm() error {
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
	).Run()
}

func validateName(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
