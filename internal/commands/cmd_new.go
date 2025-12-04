package commands

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// NewCmd implements the new command
type NewCmd struct {
	flags *Flags
}

// NewNewCmd creates a new new command
func NewNewCmd(flags *Flags) *NewCmd {
	return &NewCmd{flags: flags}
}

// Register adds the new command to the application
func (cmd *NewCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "new",
		Usage: "new command",
		Flags: []cli.Flag{
			// Add command-specific flags here
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *NewCmd) run(ctx context.Context, c *cli.Command) error {
	log.Info().Msg("running new command")

	fmt.Println("Hello World!")

	return nil
}
