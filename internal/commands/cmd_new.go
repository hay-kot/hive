package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/hay-kot/hive/internal/styles"
	"github.com/hay-kot/hive/internal/templates"
	"github.com/urfave/cli/v3"
)

type NewCmd struct {
	flags *Flags

	// Command-specific flags
	name     string
	remote   string
	template string
	setVals  []string
}

// NewNewCmd creates a new new command
func NewNewCmd(flags *Flags) *NewCmd {
	return &NewCmd{flags: flags}
}

// Register adds the new command to the application
func (cmd *NewCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "new",
		Usage:     "Create a new agent session",
		UsageText: "hive new [options]",
		Description: `Creates a new isolated git environment for an AI agent session.

If a recyclable session exists for the same remote, it will be reused
(reset, checkout main, pull). Otherwise, a fresh clone is created.

After setup, any matching hooks are executed and the configured spawn
command launches a terminal with the AI tool.

When --name is omitted, an interactive form prompts for input.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "name",
				Aliases:     []string{"n"},
				Usage:       "session name used in the directory path",
				Destination: &cmd.name,
			},
			&cli.StringFlag{
				Name:        "remote",
				Aliases:     []string{"r"},
				Usage:       "git remote URL (defaults to current directory's origin)",
				Destination: &cmd.remote,
			},
			&cli.StringFlag{
				Name:        "template",
				Aliases:     []string{"t"},
				Usage:       "use a session template (run 'hive templates list' to see available)",
				Destination: &cmd.template,
			},
			&cli.StringSliceFlag{
				Name:        "set",
				Usage:       "set template field value (name=value), use commas for multi-select",
				Destination: &cmd.setVals,
			},
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *NewCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	// Determine which template to use (default to "default")
	templateName := cmd.template
	if templateName == "" {
		templateName = "default"
	}

	// Look up template from config (always succeeds due to built-in default)
	tmpl, ok := cmd.flags.Config.Templates[templateName]
	if !ok {
		return fmt.Errorf("template %q not found (run 'hive templates list' to see available)", templateName)
	}

	// Build prefilled values: parse --set flags first, then override with --name if provided
	prefilled, err := templates.ParseSetValues(cmd.setVals)
	if err != nil {
		return fmt.Errorf("parse --set values: %w", err)
	}

	// --name flag takes precedence over --set name=...
	if cmd.name != "" {
		prefilled["name"] = cmd.name
	}

	var values map[string]any

	// Determine form behavior based on prefilled values
	switch {
	case templates.AllFieldsPrefilled(tmpl, prefilled):
		// Skip form, validate required fields, use prefilled values directly
		if err := templates.ValidateRequiredFields(tmpl, prefilled); err != nil {
			return err
		}
		values = prefilled
	case len(tmpl.Fields) > 0:
		// Run form with prefilled values as defaults
		fmt.Println(styles.BannerStyle.Render(styles.Banner))
		fmt.Println()

		result, err := templates.RunForm(tmpl, prefilled)
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return fmt.Errorf("form: %w", err)
		}
		values = result.Values
	default:
		values = prefilled
	}

	// Render prompt from template
	renderedPrompt, err := templates.RenderPrompt(tmpl, values)
	if err != nil {
		return fmt.Errorf("render prompt: %w", err)
	}

	// Determine session name
	sessionName, err := cmd.resolveSessionName(tmpl, values)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	opts := hive.CreateOptions{
		Name:   sessionName,
		Remote: cmd.remote,
		Prompt: renderedPrompt,
	}

	sess, err := cmd.flags.Service.CreateSession(ctx, opts)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	p.Success("Session created", sess.Path)

	return nil
}

// resolveSessionName determines the session name from values or template.
func (cmd *NewCmd) resolveSessionName(tmpl config.Template, values map[string]any) (string, error) {
	// First check if name is in values (from form or --set/--name flags)
	if nameVal, ok := values["name"]; ok {
		if name, ok := nameVal.(string); ok && strings.TrimSpace(name) != "" {
			return name, nil
		}
	}

	// Try rendering from template's Name field
	if tmpl.Name != "" {
		name, err := templates.RenderName(tmpl, values)
		if err != nil {
			return "", fmt.Errorf("render session name: %w", err)
		}
		if strings.TrimSpace(name) != "" {
			return name, nil
		}
	}

	// Fallback: prompt user for name
	return cmd.promptForName()
}

func (cmd *NewCmd) promptForName() (string, error) {
	var name string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Session name").
				Description("Used in the directory path").
				Validate(validateName).
				Value(&name),
		),
	).WithTheme(styles.FormTheme()).Run()
	if err != nil {
		return "", fmt.Errorf("name form: %w", err)
	}
	return name, nil
}

func validateName(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
