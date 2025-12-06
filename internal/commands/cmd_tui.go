package commands

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v3"

	"github.com/hay-kot/hive/internal/tui"
)

type TuiCmd struct {
	flags        *Flags
	showAll      bool
	hideRecycled bool
}

// NewTuiCmd creates a new tui command
func NewTuiCmd(flags *Flags) *TuiCmd {
	return &TuiCmd{flags: flags}
}

// Register adds the tui command to the application
func (cmd *TuiCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "tui",
		Usage:     "Launch the interactive session manager",
		UsageText: "hive tui [--all]",
		Description: `Opens a terminal UI for managing sessions.

Navigate with arrow keys or j/k. Press r to recycle, d to delete.
Custom keybindings can be configured in the config file.

This is the default command when hive is run without arguments.

By default, only sessions for the current repository are shown.
Use --all to show all sessions, or press 'a' to toggle in the TUI.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "all",
				Usage:       "Show all sessions instead of only local repository sessions",
				Value:       false,
				Destination: &cmd.showAll,
			},
			&cli.BoolFlag{
				Name:        "hide-recycled",
				Usage:       "Hide recycled sessions (toggle with 'x' in TUI)",
				Value:       true,
				Destination: &cmd.hideRecycled,
			},
		},
		Action: cmd.run,
	})

	return app
}

// Run executes the TUI. Exported for use as default command.
func (cmd *TuiCmd) Run(ctx context.Context, c *cli.Command) error {
	return cmd.run(ctx, c)
}

func (cmd *TuiCmd) run(ctx context.Context, _ *cli.Command) error {
	// Detect current repository remote for filtering
	var localRemote string
	if !cmd.showAll {
		remote, err := cmd.flags.Service.DetectRemote(ctx, ".")
		if err == nil {
			localRemote = remote
		}
		// If detection fails (not in a git repo), show all sessions
	}

	opts := tui.Options{
		ShowAll:      cmd.showAll,
		LocalRemote:  localRemote,
		HideRecycled: cmd.hideRecycled,
	}

	m := tui.New(cmd.flags.Service, cmd.flags.Config, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}
