package commands

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// TuiCmd implements the tui command
type TuiCmd struct {
	flags *Flags
}

// NewTuiCmd creates a new tui command
func NewTuiCmd(flags *Flags) *TuiCmd {
	return &TuiCmd{flags: flags}
}

// Register adds the tui command to the application
func (cmd *TuiCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "tui",
		Usage: "tui command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *TuiCmd) run(ctx context.Context, c *cli.Command) error {
	log.Info().Msg("running tui command")

	fmt.Println("Hello World!")

	return nil
}
