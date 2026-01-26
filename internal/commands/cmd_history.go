package commands

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

type HistoryCmd struct {
	flags *Flags

	// Command-specific flags
	clear bool
}

// NewHistoryCmd creates a new history command
func NewHistoryCmd(flags *Flags) *HistoryCmd {
	return &HistoryCmd{flags: flags}
}

// Register adds the history command to the application
func (cmd *HistoryCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "history",
		Usage:     "View or manage command history",
		UsageText: "hive history [options]",
		Description: `View or manage the history of 'new' commands.

By default, lists recent commands with their IDs, status, and timestamp.
Use --clear to remove all history entries.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "clear",
				Aliases:     []string{"c"},
				Usage:       "clear all command history",
				Destination: &cmd.clear,
			},
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *HistoryCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	if cmd.clear {
		return cmd.runClear(ctx, p)
	}

	return cmd.runList(ctx, c)
}

func (cmd *HistoryCmd) runList(ctx context.Context, c *cli.Command) error {
	entries, err := cmd.flags.HistoryStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list history: %w", err)
	}

	if len(entries) == 0 {
		printer.Ctx(ctx).Infof("No command history")
		return nil
	}

	out := c.Root().Writer
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tCOMMAND\tSTATUS\tTIME")

	for _, e := range entries {
		status := printer.StatusOK()
		if e.Failed() {
			status = printer.StatusFailed(fmt.Sprintf("exit %d", e.ExitCode))
		}

		cmdStr := e.CommandString()
		if len(cmdStr) > 50 {
			cmdStr = cmdStr[:47] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			e.ID,
			cmdStr,
			status,
			e.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}

	return w.Flush()
}

func (cmd *HistoryCmd) runClear(ctx context.Context, p *printer.Printer) error {
	if err := cmd.flags.HistoryStore.Clear(ctx); err != nil {
		return fmt.Errorf("clear history: %w", err)
	}

	p.Successf("Command history cleared")
	return nil
}
