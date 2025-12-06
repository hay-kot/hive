package commands

import (
	"context"
	"encoding/json"

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

	result := cmd.flags.Config.ValidateDeep(cmd.flags.ConfigPath)

	if cmd.format == "json" {
		return cmd.outputJSON(c, result)
	}

	return cmd.outputText(p, result)
}

func (cmd *ConfigValidateCmd) outputJSON(c *cli.Command, result *config.ValidationResult) error {
	out := struct {
		Valid    bool                       `json:"valid"`
		Errors   []config.ValidationError   `json:"errors,omitempty"`
		Warnings []config.ValidationWarning `json:"warnings,omitempty"`
		Checks   []config.ValidationCheck   `json:"checks,omitempty"`
	}{
		Valid:    result.IsValid(),
		Errors:   result.Errors,
		Warnings: result.Warnings,
		Checks:   result.Checks,
	}

	enc := json.NewEncoder(c.Root().Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (cmd *ConfigValidateCmd) outputText(p *printer.Printer, result *config.ValidationResult) error {
	// Print successful checks
	for _, check := range result.Checks {
		p.Successf("%s: %s", check.Category, check.Message)
		for _, detail := range check.Details {
			p.Printf("  %s", detail)
		}
	}

	// Print warnings
	for _, warn := range result.Warnings {
		p.Infof("%s: %s", warn.Category, warn.Message)
		if warn.Item != "" {
			p.Printf("  Item: %s", warn.Item)
		}
	}

	// Print errors
	for _, err := range result.Errors {
		p.Errorf("%s: %s", err.Category, err.Message)
		if err.Item != "" {
			p.Printf("  Item: %s", err.Item)
		}
		if err.Fix != "" {
			p.Printf("  Fix: %s", err.Fix)
		}
	}

	// Print summary
	p.Printf("")
	if result.IsValid() {
		p.Successf("Configuration is valid")
		return nil
	}

	p.Errorf("%d error(s) found", result.ErrorCount())
	return cli.Exit("", 1)
}
