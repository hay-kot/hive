package hive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"text/template"

	"github.com/hay-kot/hive/pkg/executil"
	"github.com/rs/zerolog"
)

// RecycleData contains template data for recycle commands.
type RecycleData struct {
	DefaultBranch string
}

// Recycler handles resetting a session environment for reuse.
type Recycler struct {
	log      zerolog.Logger
	executor executil.Executor
}

// NewRecycler creates a new Recycler.
func NewRecycler(log zerolog.Logger, executor executil.Executor) *Recycler {
	return &Recycler{
		log:      log,
		executor: executor,
	}
}

// Recycle executes recycle commands sequentially in the session directory.
// Commands are rendered as Go templates with the provided data.
// Output is written to the provided writer. If w is nil, output is discarded.
func (r *Recycler) Recycle(ctx context.Context, path string, commands []string, data RecycleData, w io.Writer) error {
	r.log.Debug().Str("path", path).Msg("recycling environment")

	if w == nil {
		w = io.Discard
	}

	for _, cmd := range commands {
		// Render template
		rendered, err := r.renderCommand(cmd, data)
		if err != nil {
			return fmt.Errorf("render command %q: %w", cmd, err)
		}

		r.log.Debug().Str("command", rendered).Msg("executing recycle command")

		if err := r.executor.RunDirStream(ctx, path, w, w, "sh", "-c", rendered); err != nil {
			return fmt.Errorf("execute recycle command %q: %w", rendered, err)
		}
	}

	r.log.Debug().Msg("recycle complete")
	return nil
}

func (r *Recycler) renderCommand(cmd string, data RecycleData) (string, error) {
	tmpl, err := template.New("cmd").Parse(cmd)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
