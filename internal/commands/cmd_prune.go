package commands

import (
	"context"
	"fmt"

	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

// PruneCmd implements the prune command
type PruneCmd struct {
	flags *Flags
}

// NewPruneCmd creates a new prune command
func NewPruneCmd(flags *Flags) *PruneCmd {
	return &PruneCmd{flags: flags}
}

// Register adds the prune command to the application
func (cmd *PruneCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "prune",
		Usage: "Remove all recycled sessions and their directories",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *PruneCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	count, err := cmd.flags.Service.Prune(ctx)
	if err != nil {
		return fmt.Errorf("prune sessions: %w", err)
	}

	if count == 0 {
		p.Infof("No recycled sessions to prune")
		return nil
	}

	p.Successf("Pruned %d session(s)", count)

	return nil
}
