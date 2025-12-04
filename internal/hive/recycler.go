package hive

import (
	"context"
	"fmt"
	"io"

	"github.com/hay-kot/hive/pkg/executil"
	"github.com/rs/zerolog"
)

// Recycler handles resetting a session environment for reuse.
type Recycler struct {
	log      zerolog.Logger
	executor executil.Executor
	stdout   io.Writer
	stderr   io.Writer
}

// NewRecycler creates a new Recycler.
func NewRecycler(log zerolog.Logger, executor executil.Executor, stdout, stderr io.Writer) *Recycler {
	return &Recycler{
		log:      log,
		executor: executor,
		stdout:   stdout,
		stderr:   stderr,
	}
}

// Recycle executes recycle commands sequentially in the session directory.
func (r *Recycler) Recycle(ctx context.Context, path string, commands []string) error {
	r.log.Debug().Str("path", path).Msg("recycling environment")

	for _, cmd := range commands {
		r.log.Debug().Str("command", cmd).Msg("executing recycle command")

		if err := r.executor.RunDirStream(ctx, path, r.stdout, r.stderr, "sh", "-c", cmd); err != nil {
			return fmt.Errorf("execute recycle command %q: %w", cmd, err)
		}
	}

	r.log.Debug().Msg("recycle complete")
	return nil
}
