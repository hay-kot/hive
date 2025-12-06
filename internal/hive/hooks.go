package hive

import (
	"context"
	"fmt"
	"io"
	"regexp"
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

// RunHooks executes hooks matching the remote URL.
func (h *HookRunner) RunHooks(ctx context.Context, hooks []config.Hook, remote, path string) error {
	h.log.Debug().
		Str("remote", remote).
		Int("hook_count", len(hooks)).
		Msg("evaluating hooks")

	hookNum := 0
	for _, hook := range hooks {
		matched, err := matchPattern(hook.Pattern, remote)
		if err != nil {
			return fmt.Errorf("match pattern %q: %w", hook.Pattern, err)
		}

		h.log.Debug().
			Str("pattern", hook.Pattern).
			Str("remote", remote).
			Bool("matched", matched).
			Msg("hook pattern evaluated")

		if !matched {
			continue
		}

		hookNum++

		h.log.Debug().
			Str("pattern", hook.Pattern).
			Strs("commands", hook.Commands).
			Msg("running hook")

		for i, cmd := range hook.Commands {
			h.printCommandHeader(hookNum, i+1, len(hook.Commands), cmd)

			if err := h.executor.RunDirStream(ctx, path, h.stdout, h.stderr, "sh", "-c", cmd); err != nil {
				return fmt.Errorf("run hook %q command %q: %w", hook.Pattern, cmd, err)
			}

			_, _ = fmt.Fprintln(h.stdout)
		}
	}

	return nil
}

// printCommandHeader prints a styled header for a hook command.
func (h *HookRunner) printCommandHeader(hookNum, cmdNum, totalCmds int, cmd string) {
	divider := styles.DividerStyle.Render(strings.Repeat("─", 50))
	header := styles.CommandHeaderStyle.Render(fmt.Sprintf("hook %d", hookNum))
	cmdLabel := styles.DividerStyle.Render(fmt.Sprintf("[%d/%d]", cmdNum, totalCmds))
	command := styles.CommandStyle.Render(cmd)

	_, _ = fmt.Fprintln(h.stdout)
	_, _ = fmt.Fprintln(h.stdout, divider)
	_, _ = fmt.Fprintf(h.stdout, "%s %s %s\n", header, cmdLabel, command)
	_, _ = fmt.Fprintln(h.stdout, divider)
}

// matchPattern checks if remote matches the regex pattern.
func matchPattern(pattern, remote string) (bool, error) {
	return regexp.MatchString(pattern, remote)
}
