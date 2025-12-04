package hive

import (
	"context"
	"fmt"
	"io"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/hay-kot/hive/internal/core/config"
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

		h.log.Debug().
			Str("pattern", hook.Pattern).
			Strs("commands", hook.Commands).
			Msg("running hook")

		for _, cmd := range hook.Commands {
			if err := h.executor.RunDirStream(ctx, path, h.stdout, h.stderr, "sh", "-c", cmd); err != nil {
				return fmt.Errorf("run hook %q command %q: %w", hook.Pattern, cmd, err)
			}
		}
	}

	return nil
}

// matchPattern checks if remote matches the glob pattern using doublestar.
// Supports ** for matching across path separators.
func matchPattern(pattern, remote string) (bool, error) {
	return doublestar.Match(pattern, remote)
}
