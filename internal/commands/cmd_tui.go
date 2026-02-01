package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/urfave/cli/v3"

	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/internal/integration/terminal"
	"github.com/hay-kot/hive/internal/integration/terminal/tmux"
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

	// Create terminal integration manager if configured
	var termMgr *terminal.Manager
	if len(cmd.flags.Config.Integrations.Terminal.Enabled) > 0 {
		termMgr = terminal.NewManager(cmd.flags.Config.Integrations.Terminal.Enabled)
		// Register tmux integration
		tmuxIntegration := tmux.New()
		if tmuxIntegration.Available() {
			termMgr.Register(tmuxIntegration)
		}
	}

	for {
		opts := tui.Options{
			LocalRemote:     localRemote,
			MsgStore:        msgStore,
			TerminalManager: termMgr,
		}

		m := tui.New(cmd.flags.Service, cmd.flags.Config, opts)
		p := tea.NewProgram(m)

		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("run tui: %w", err)
		}

		model := finalModel.(tui.Model)

		// Handle pending session creation
		if pending := model.PendingCreate(); pending != nil {
			source, _ := os.Getwd()
			_, err := cmd.flags.Service.CreateSession(ctx, hive.CreateOptions{
				Name:   pending.Name,
				Remote: pending.Remote,
				Source: source,
			})
			if err != nil {
				fmt.Printf("Error creating session: %v\n", err)
				fmt.Println("Press Enter to continue...")
				_, _ = fmt.Scanln()
			}
			continue // Restart TUI
		}

		break // Normal exit
	}

	return nil
}
