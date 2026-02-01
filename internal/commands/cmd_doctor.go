package commands

import (
	"context"
	"encoding/json"

	"github.com/hay-kot/hive/internal/commands/doctor"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/urfave/cli/v3"
)

type DoctorCmd struct {
	flags   *Flags
	format  string
	autofix bool
}

func NewDoctorCmd(flags *Flags) *DoctorCmd {
	return &DoctorCmd{flags: flags}
}

func (cmd *DoctorCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:        "doctor",
		Usage:       "Run health checks on your hive setup",
		UsageText:   "hive doctor [options]",
		Description: "Runs diagnostic checks on configuration, environment, and dependencies.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "format",
				Usage:       "output format (text, json)",
				Value:       "text",
				Destination: &cmd.format,
			},
			&cli.BoolFlag{
				Name:        "autofix",
				Usage:       "automatically fix issues (e.g., delete orphaned worktrees)",
				Destination: &cmd.autofix,
			},
		},
		Action: cmd.run,
	})
	return app
}

func (cmd *DoctorCmd) run(ctx context.Context, c *cli.Command) error {
	checks := []doctor.Check{
		doctor.NewConfigCheck(cmd.flags.Config, cmd.flags.ConfigPath),
		doctor.NewOrphanCheck(cmd.flags.Store, cmd.flags.Config.ReposDir(), cmd.autofix),
	}

	results := doctor.RunAll(ctx, checks)

	if cmd.format == "json" {
		return cmd.outputJSON(c, results)
	}

	return cmd.outputText(ctx, results)
}

func (cmd *DoctorCmd) outputJSON(c *cli.Command, results []doctor.Result) error {
	passed, warned, failed := doctor.Summary(results)

	out := struct {
		Healthy bool            `json:"healthy"`
		Summary summaryJSON     `json:"summary"`
		Checks  []doctor.Result `json:"checks"`
	}{
		Healthy: failed == 0,
		Summary: summaryJSON{Passed: passed, Warned: warned, Failed: failed},
		Checks:  results,
	}

	enc := json.NewEncoder(c.Root().Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

type summaryJSON struct {
	Passed int `json:"passed"`
	Warned int `json:"warned"`
	Failed int `json:"failed"`
}

func (cmd *DoctorCmd) outputText(ctx context.Context, results []doctor.Result) error {
	p := printer.Ctx(ctx)

	for _, result := range results {
		p.Section(result.Name)

		for _, item := range result.Items {
			switch item.Status {
			case doctor.StatusPass:
				p.CheckItem(item.Label, item.Detail)
			case doctor.StatusWarn:
				p.WarnItem(item.Label, item.Detail)
			case doctor.StatusFail:
				p.FailItem(item.Label, item.Detail)
			}
		}

		p.Printf("")
	}

	passed, warned, failed := doctor.Summary(results)
	p.Printf("Summary: %d passed, %d warnings, %d failed", passed, warned, failed)

	// Show hint if there are fixable issues and autofix wasn't used
	if !cmd.autofix {
		fixable := doctor.CountFixable(results)
		if fixable > 0 {
			p.Printf("")
			p.Printf("Run 'hive doctor --autofix' to fix %d issue(s)", fixable)
		}
	}

	if failed > 0 {
		return cli.Exit("", 1)
	}

	return nil
}
