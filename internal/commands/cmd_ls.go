package commands

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

type LsCmd struct {
	flags *Flags
}

// NewLsCmd creates a new ls command
func NewLsCmd(flags *Flags) *LsCmd {
	return &LsCmd{flags: flags}
}

// Register adds the ls command to the application
func (cmd *LsCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:        "ls",
		Usage:       "List all sessions",
		UsageText:   "hive ls",
		Description: "Displays a table of all sessions with their ID, name, state, and path.",
		Action:      cmd.run,
	})

	return app
}

func (cmd *LsCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	sessions, err := cmd.flags.Service.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		p.Infof("No sessions found")
		return nil
	}

	// Separate normal and corrupted sessions
	var normal, corrupted []session.Session
	for _, s := range sessions {
		if s.State == session.StateCorrupted {
			corrupted = append(corrupted, s)
		} else {
			normal = append(normal, s)
		}
	}

	out := c.Root().Writer

	if len(normal) > 0 {
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tNAME\tSTATE\tPATH")

		for _, s := range normal {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, s.Name, s.State, s.Path)
		}

		_ = w.Flush()
	}

	if len(corrupted) > 0 {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "Corrupted sessions (run 'hive prune' to clean up):")
		for _, s := range corrupted {
			_, _ = fmt.Fprintf(out, "  %s\t%s\n", s.ID, s.Path)
		}
	}

	return nil
}
