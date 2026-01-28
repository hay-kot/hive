// Package hive provides the service layer for orchestrating hive operations.
package hive

import (
	"context"
	"fmt"
	"io"

	"github.com/hay-kot/hive/pkg/executil"
	"github.com/hay-kot/hive/pkg/tmpl"
	"github.com/rs/zerolog"
)

// SpawnData is the template context for spawn commands.
type SpawnData struct {
	Path       string // Absolute path to session directory
	Name       string // Session name (display name)
	Slug       string // Session slug (URL-safe version of name)
	Prompt     string // AI prompt
	ContextDir string // Path to context directory
	Owner      string // Repository owner
	Repo       string // Repository name
}

// Spawner handles terminal spawning with template rendering.
type Spawner struct {
	log      zerolog.Logger
	executor executil.Executor
	stdout   io.Writer
	stderr   io.Writer
}

// NewSpawner creates a new Spawner.
func NewSpawner(log zerolog.Logger, executor executil.Executor, stdout, stderr io.Writer) *Spawner {
	return &Spawner{
		log:      log,
		executor: executor,
		stdout:   stdout,
		stderr:   stderr,
	}
}

// Spawn executes spawn commands sequentially with template rendering.
func (s *Spawner) Spawn(ctx context.Context, commands []string, data SpawnData) error {
	for _, cmdTmpl := range commands {
		s.log.Debug().Str("command", cmdTmpl).Msg("executing spawn command")

		rendered, err := tmpl.Render(cmdTmpl, data)
		if err != nil {
			return fmt.Errorf("render spawn command %q: %w", cmdTmpl, err)
		}

		if err := s.executor.RunStream(ctx, s.stdout, s.stderr, "sh", "-c", rendered); err != nil {
			return fmt.Errorf("execute spawn command %q: %w", rendered, err)
		}
	}

	s.log.Debug().Msg("spawn complete")
	return nil
}
