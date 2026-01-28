package hive

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/styles"
	"github.com/hay-kot/hive/pkg/executil"
	"github.com/rs/zerolog"
)

// HookRunner executes repository-specific setup hooks.
type HookRunner struct {
	log      zerolog.Logger
	executor executil.Executor
	stdout   io.Writer
	stderr   io.Writer
}

// NewHookRunner creates a new HookRunner.
func NewHookRunner(log zerolog.Logger, executor executil.Executor, stdout, stderr io.Writer) *HookRunner {
	return &HookRunner{
		log:      log,
		executor: executor,
		stdout:   stdout,
		stderr:   stderr,
	}
}

// RunHooks executes the commands from a matched rule.
func (h *HookRunner) RunHooks(ctx context.Context, rule config.Rule, path string) error {
	h.log.Debug().
		Str("pattern", rule.Pattern).
		Strs("commands", rule.Commands).
		Msg("running rule commands")

	for i, cmd := range rule.Commands {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		h.printCommandHeader(i+1, len(rule.Commands), cmd)

		if err := h.executor.RunDirStream(ctx, path, h.stdout, h.stderr, "sh", "-c", cmd); err != nil {
			return fmt.Errorf("run command %q: %w", cmd, err)
		}

		_, _ = fmt.Fprintln(h.stdout)
	}

	return nil
}

// printCommandHeader prints a styled header for a hook command.
func (h *HookRunner) printCommandHeader(cmdNum, totalCmds int, cmd string) {
	divider := styles.DividerStyle.Render(strings.Repeat("─", 50))
	header := styles.CommandHeaderStyle.Render("hook")
	cmdLabel := styles.DividerStyle.Render(fmt.Sprintf("[%d/%d]", cmdNum, totalCmds))
	command := styles.CommandStyle.Render(cmd)

	_, _ = fmt.Fprintln(h.stdout)
	_, _ = fmt.Fprintln(h.stdout, divider)
	_, _ = fmt.Fprintf(h.stdout, "%s %s %s\n", header, cmdLabel, command)
	_, _ = fmt.Fprintln(h.stdout, divider)
}
