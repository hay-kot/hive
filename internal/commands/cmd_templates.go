package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

type TemplatesCmd struct {
	flags *Flags
}

// NewTemplatesCmd creates a new templates command
func NewTemplatesCmd(flags *Flags) *TemplatesCmd {
	return &TemplatesCmd{flags: flags}
}

// Register adds the templates command to the application
func (cmd *TemplatesCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:        "templates",
		Usage:       "Manage session templates",
		UsageText:   "hive templates <command>",
		Description: "List and inspect session templates defined in your config file.",
		Commands: []*cli.Command{
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Usage:       "List all available templates",
				UsageText:   "hive templates list",
				Description: "Displays a table of all templates with their name, description, and field count.",
				Action:      cmd.runList,
			},
			{
				Name:        "show",
				Usage:       "Show details of a specific template",
				UsageText:   "hive templates show <name>",
				Description: "Displays the full details of a template including all fields and the prompt template.",
				Action:      cmd.runShow,
			},
		},
	})

	return app
}

func (cmd *TemplatesCmd) runList(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	templates := cmd.flags.Config.Templates
	if len(templates) == 0 {
		p.Infof("No templates defined. Add templates to your config file.")
		return nil
	}

	// Get sorted template names for consistent output
	names := make([]string, 0, len(templates))
	for name := range templates {
		names = append(names, name)
	}
	sort.Strings(names)

	w := tabwriter.NewWriter(c.Root().Writer, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION\tFIELDS")

	for _, name := range names {
		tmpl := templates[name]
		desc := tmpl.Description
		if desc == "" {
			desc = "-"
		}
		// Truncate long descriptions
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\n", name, desc, len(tmpl.Fields))
	}

	return w.Flush()
}

func (cmd *TemplatesCmd) runShow(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	if c.Args().Len() < 1 {
		return fmt.Errorf("template name required")
	}

	name := c.Args().First()
	tmpl, ok := cmd.flags.Config.Templates[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}

	w := c.Root().Writer

	// Print template header
	p.Infof("Template: %s", name)
	if tmpl.Description != "" {
		_, _ = fmt.Fprintf(w, "Description: %s\n", tmpl.Description)
	}
	_, _ = fmt.Fprintln(w)

	// Print fields
	if len(tmpl.Fields) > 0 {
		_, _ = fmt.Fprintln(w, "Fields:")
		for _, field := range tmpl.Fields {
			printField(w, field)
		}
		_, _ = fmt.Fprintln(w)
	} else {
		_, _ = fmt.Fprintln(w, "Fields: (none)")
		_, _ = fmt.Fprintln(w)
	}

	// Print name template if present
	if tmpl.Name != "" {
		_, _ = fmt.Fprintln(w, "Session Name Template:")
		_, _ = fmt.Fprintf(w, "  %s\n\n", tmpl.Name)
	}

	// Print prompt template
	_, _ = fmt.Fprintln(w, "Prompt Template:")
	// Indent each line of the prompt
	for _, line := range strings.Split(tmpl.Prompt, "\n") {
		_, _ = fmt.Fprintf(w, "  %s\n", line)
	}

	return nil
}

func printField(w writer, field config.TemplateField) {
	required := ""
	if field.Required {
		required = " (required)"
	}

	label := field.Label
	if label == "" {
		label = field.Name
	}

	_, _ = fmt.Fprintf(w, "  • %s%s\n", label, required)
	_, _ = fmt.Fprintf(w, "    Name: %s, Type: %s\n", field.Name, field.Type)

	if field.Default != "" {
		_, _ = fmt.Fprintf(w, "    Default: %s\n", field.Default)
	}

	if field.Placeholder != "" {
		_, _ = fmt.Fprintf(w, "    Placeholder: %s\n", field.Placeholder)
	}

	if len(field.Options) > 0 {
		opts := make([]string, len(field.Options))
		for i, opt := range field.Options {
			if opt.Label != "" && opt.Label != opt.Value {
				opts[i] = fmt.Sprintf("%s (%s)", opt.Value, opt.Label)
			} else {
				opts[i] = opt.Value
			}
		}
		_, _ = fmt.Fprintf(w, "    Options: %s\n", strings.Join(opts, ", "))
	}
}

type writer interface {
	Write(p []byte) (n int, err error)
}
