package commands

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

// LsCmd implements the ls command
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
		Name:  "ls",
		Usage: "List all sessions",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
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

	w := tabwriter.NewWriter(c.Root().Writer, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tSTATE\tPATH")

	for _, s := range sessions {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, s.Name, s.State, s.Path)
	}

	return w.Flush()
}
