package commands

import (
	"context"
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v3"

	"github.com/hay-kot/hive/internal/store/jsonfile"
	"github.com/hay-kot/hive/internal/tui"
)

type TuiCmd struct {
	flags        *Flags
	showAll      bool
	hideRecycled bool
}

// NewTuiCmd creates a new tui command
func NewTuiCmd(flags *Flags) *TuiCmd {
	return &TuiCmd{
		flags:        flags,
		hideRecycled: true,
	}
}

// Flags returns the TUI-specific flags for registration on the root command
func (cmd *TuiCmd) Flags() []cli.Flag {
	return []cli.Flag{
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
	}
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

	// Create message store for pub/sub events
	topicsDir := filepath.Join(cmd.flags.DataDir, "messages", "topics")
	msgStore := jsonfile.NewMsgStore(topicsDir)

	opts := tui.Options{
		ShowAll:      cmd.showAll,
		LocalRemote:  localRemote,
		HideRecycled: cmd.hideRecycled,
		MsgStore:     msgStore,
	}

	m := tui.New(cmd.flags.Service, cmd.flags.Config, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}
