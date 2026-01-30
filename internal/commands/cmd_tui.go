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
	flags *Flags
}

// NewTuiCmd creates a new tui command
func NewTuiCmd(flags *Flags) *TuiCmd {
	return &TuiCmd{
		flags: flags,
	}
}

// Flags returns the TUI-specific flags for registration on the root command
func (cmd *TuiCmd) Flags() []cli.Flag {
	return nil
}

// Run executes the TUI. Exported for use as default command.
func (cmd *TuiCmd) Run(ctx context.Context, c *cli.Command) error {
	return cmd.run(ctx, c)
}

func (cmd *TuiCmd) run(ctx context.Context, _ *cli.Command) error {
	// Detect current repository remote for highlighting current repo
	localRemote, _ := cmd.flags.Service.DetectRemote(ctx, ".")

	// Create message store for pub/sub events
	topicsDir := filepath.Join(cmd.flags.DataDir, "messages", "topics")
	msgStore := jsonfile.NewMsgStore(topicsDir)

	opts := tui.Options{
		LocalRemote: localRemote,
		MsgStore:    msgStore,
	}

	m := tui.New(cmd.flags.Service, cmd.flags.Config, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}
