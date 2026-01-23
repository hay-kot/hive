package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hay-kot/criterio"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

type ConfigValidateCmd struct {
	flags  *Flags
	format string
}

// NewConfigValidateCmd creates a new config validate command.
func NewConfigValidateCmd(flags *Flags) *ConfigValidateCmd {
	return &ConfigValidateCmd{flags: flags}
}

// Register adds the config validate command to the application.
func (cmd *ConfigValidateCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "config",
		Usage: "Configuration management commands",
		Commands: []*cli.Command{
			{
				Name:        "validate",
				Usage:       "Validate configuration file",
				UsageText:   "hive config validate [options]",
				Description: "Validates the configuration file, checking template syntax, regex patterns, and file paths.",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "format",
						Usage:       "output format (text, json)",
						Value:       "text",
						Destination: &cmd.format,
					},
				},
				Action: cmd.run,
			},
		},
	})

	return app
}

func (cmd *ConfigValidateCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	if cmd.flags.Config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	err := cmd.flags.Config.ValidateDeep(cmd.flags.ConfigPath)
	warnings := cmd.flags.Config.Warnings()

	if cmd.format == "json" {
		return cmd.outputJSON(c, err, warnings)
	}

	return cmd.outputText(p, err, warnings)
}

func (cmd *ConfigValidateCmd) outputJSON(c *cli.Command, validationErr error, warnings []config.ValidationWarning) error {
	type fieldError struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}

	out := struct {
		Valid    bool                       `json:"valid"`
		Errors   []fieldError               `json:"errors,omitempty"`
		Warnings []config.ValidationWarning `json:"warnings,omitempty"`
	}{
		Valid:    validationErr == nil,
		Warnings: warnings,
	}

	for _, fe := range extractFieldErrors(validationErr) {
		out.Errors = append(out.Errors, fieldError{Field: fe.Field, Message: fe.Err.Error()})
	}

	enc := json.NewEncoder(c.Root().Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// extractFieldErrors extracts field errors from a validation error.
func extractFieldErrors(err error) criterio.FieldErrors {
	if err == nil {
		return nil
	}
	var fieldErrs criterio.FieldErrors
	if errors.As(err, &fieldErrs) {
		return fieldErrs
	}
	return criterio.FieldErrors{{Err: err}}
}

func (cmd *ConfigValidateCmd) outputText(p *printer.Printer, validationErr error, warnings []config.ValidationWarning) error {
	fieldErrs := extractFieldErrors(validationErr)

	if len(fieldErrs) > 0 {
		p.Printf("Errors")
		for _, fe := range fieldErrs {
			if fe.Field != "" {
				p.Printf("  %s %s: %s", printer.Cross, fe.Field, fe.Err.Error())
			} else {
				p.Printf("  %s %s", printer.Cross, fe.Err.Error())
			}
		}
	}

	if len(warnings) > 0 {
		if len(fieldErrs) > 0 {
			p.Printf("")
		}
		p.Printf("Warnings")
		for _, warn := range warnings {
			msg := warn.Message
			if warn.Item != "" {
				msg = warn.Item + ": " + msg
			}
			p.Printf("  %s %s: %s", printer.Dot, warn.Category, msg)
		}
	}

	p.Printf("")
	if validationErr == nil {
		if len(warnings) > 0 {
			p.Successf("Configuration is valid (%d warning(s))", len(warnings))
		} else {
			p.Successf("Configuration is valid")
		}
		return nil
	}

	p.Errorf("%d error(s), %d warning(s)", len(fieldErrs), len(warnings))
	return cli.Exit("", 1)
}
