package commands

import (
	"context"
	"fmt"

	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

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
		Name:      "prune",
		Usage:     "Remove recycled sessions exceeding max_recycled limit",
		UsageText: "hive prune [--all]",
		Description: `Removes recycled sessions based on the max_recycled configuration.

By default, keeps the newest N recycled sessions per repository (based on
max_recycled config) and deletes the rest.

Use --all to delete ALL recycled sessions regardless of the limit.

Active sessions are not affected.`,
		Action: cmd.run,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Delete all recycled sessions (ignore max_recycled limit)",
			},
		},
	})

	return app
}

func (cmd *PruneCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	all := c.Bool("all")
	count, err := cmd.flags.Service.Prune(ctx, all)
	if err != nil {
		return fmt.Errorf("prune sessions: %w", err)
	}

	if count == 0 {
		if all {
			p.Infof("No recycled sessions to prune")
		} else {
			p.Infof("No sessions exceed max_recycled limit")
		}
		return nil
	}

	p.Successf("Pruned %d session(s)", count)

	return nil
}
