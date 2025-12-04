package commands

import (
	"context"
	"fmt"

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
				Required:    true,
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
