package hive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/git"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/pkg/executil"
	"github.com/hay-kot/hive/pkg/randid"
	"github.com/rs/zerolog"
)

// CreateOptions configures session creation.
type CreateOptions struct {
	Name          string // Session name (used in path)
	Prompt        string // Prompt to pass to spawned terminal (batch only)
	Remote        string // Git remote URL to clone (auto-detected if empty)
	Source        string // Source directory for file copying
	UseBatchSpawn bool   // Use batch_spawn commands instead of spawn
}

// Service orchestrates hive operations.
type Service struct {
	sessions   session.Store
	git        git.Git
	config     *config.Config
	executor   executil.Executor
	log        zerolog.Logger
	spawner    *Spawner
	recycler   *Recycler
	hookRunner *HookRunner
	fileCopier *FileCopier
}

// New creates a new Service.
func New(
	sessions session.Store,
	gitClient git.Git,
	cfg *config.Config,
	exec executil.Executor,
	log zerolog.Logger,
	stdout, stderr io.Writer,
) *Service {
	return &Service{
		sessions:   sessions,
		git:        gitClient,
		config:     cfg,
		executor:   exec,
		log:        log,
		spawner:    NewSpawner(log.With().Str("component", "spawner").Logger(), exec, stdout, stderr),
		recycler:   NewRecycler(log.With().Str("component", "recycler").Logger(), exec),
		hookRunner: NewHookRunner(log.With().Str("component", "hooks").Logger(), exec, stdout, stderr),
		fileCopier: NewFileCopier(log.With().Str("component", "copier").Logger(), stdout),
	}
}

// CreateSession creates a new session or recycles an existing one.
func (s *Service) CreateSession(ctx context.Context, opts CreateOptions) (*session.Session, error) {
	s.log.Info().Str("name", opts.Name).Str("remote", opts.Remote).Msg("creating session")

	remote := opts.Remote
	if remote == "" {
		var err error
		remote, err = s.DetectRemote(ctx, ".")
		if err != nil {
			return nil, fmt.Errorf("detect remote: %w", err)
		}
		s.log.Debug().Str("remote", remote).Msg("detected remote")
	}

	var sess session.Session
	slug := session.Slugify(opts.Name)

	// Try to find and validate a recyclable session
	recyclable := s.findValidRecyclable(ctx, remote)

	if recyclable != nil {
		// Reuse existing recycled session (already cleaned up when marked for recycle)
		s.log.Debug().Str("session_id", recyclable.ID).Msg("found valid recyclable session")

		// Pull latest changes before running hooks
		s.log.Debug().Str("path", recyclable.Path).Msg("pulling latest changes")
		if err := s.git.Pull(ctx, recyclable.Path); err != nil {
			// Pull failed - mark as corrupted and fall through to clone
			s.log.Warn().Err(err).Str("session_id", recyclable.ID).Msg("pull failed, marking corrupted")
			s.markCorrupted(ctx, recyclable)
			recyclable = nil
		}
	}

	if recyclable != nil {
		// Rename directory to new session name pattern
		repoName := git.ExtractRepoName(remote)
		newPath := filepath.Join(s.config.ReposDir(), fmt.Sprintf("%s-%s-%s", repoName, slug, recyclable.ID))

		if err := os.Rename(recyclable.Path, newPath); err != nil {
			return nil, fmt.Errorf("rename recycled directory: %w", err)
		}

		sess = *recyclable
		sess.Name = opts.Name
		sess.Slug = slug
		sess.Path = newPath
		sess.State = session.StateActive
		sess.UpdatedAt = time.Now()
	} else {
		// Create new session (either no recyclable found or it was corrupted)
		id := generateID()
		repoName := git.ExtractRepoName(remote)
		path := filepath.Join(s.config.ReposDir(), fmt.Sprintf("%s-%s-%s", repoName, slug, id))

		s.log.Info().Str("remote", remote).Str("dest", path).Msg("cloning repository")

		if err := s.git.Clone(ctx, remote, path); err != nil {
			return nil, fmt.Errorf("clone repository: %w", err)
		}

		s.log.Debug().Msg("clone complete")

		now := time.Now()
		sess = session.Session{
			ID:        id,
			Name:      opts.Name,
			Slug:      slug,
			Path:      path,
			Remote:    remote,
			State:     session.StateActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	// Execute matching rules
	if err := s.executeRules(ctx, remote, opts.Source, sess.Path); err != nil {
		return nil, fmt.Errorf("execute rules: %w", err)
	}

	// Save session
	if err := s.sessions.Save(ctx, sess); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	// Spawn terminal
	spawnCommands := s.config.Commands.Spawn
	if opts.UseBatchSpawn && len(s.config.Commands.BatchSpawn) > 0 {
		spawnCommands = s.config.Commands.BatchSpawn
	}

	if len(spawnCommands) > 0 {
		owner, repoName := git.ExtractOwnerRepo(remote)
		data := SpawnData{
			Path:       sess.Path,
			Name:       sess.Name,
			Prompt:     opts.Prompt,
			Slug:       sess.Slug,
			ContextDir: s.config.RepoContextDir(owner, repoName),
			Owner:      owner,
			Repo:       repoName,
		}
		if err := s.spawner.Spawn(ctx, spawnCommands, data); err != nil {
			return nil, fmt.Errorf("spawn terminal: %w", err)
		}
	}

	s.log.Info().Str("session_id", sess.ID).Str("path", sess.Path).Msg("session created")

	return &sess, nil
}

// ListSessions returns all sessions.
func (s *Service) ListSessions(ctx context.Context) ([]session.Session, error) {
	return s.sessions.List(ctx)
}

// GetSession returns a session by ID.
func (s *Service) GetSession(ctx context.Context, id string) (session.Session, error) {
	return s.sessions.Get(ctx, id)
}

// RecycleSession marks a session for recycling and runs recycle commands.
// The directory is renamed to a recycled name pattern immediately.
// Output is written to w. If w is nil, output is discarded.
func (s *Service) RecycleSession(ctx context.Context, id string, w io.Writer) error {
	sess, err := s.sessions.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if !sess.CanRecycle() {
		return fmt.Errorf("session %s cannot be recycled (state: %s)", id, sess.State)
	}

	// Validate repository before recycling
	if err := s.git.IsValidRepo(ctx, sess.Path); err != nil {
		s.log.Warn().Err(err).Str("session_id", id).Msg("session has corrupted repository")
		s.markCorrupted(ctx, &sess)
		return fmt.Errorf("session %s has corrupted repository: %w", id, err)
	}

	// Get default branch for template
	defaultBranch, err := s.git.DefaultBranch(ctx, sess.Path)
	if err != nil {
		s.log.Warn().Err(err).Msg("failed to get default branch, using 'main'")
		defaultBranch = "main"
	}

	data := RecycleData{
		DefaultBranch: defaultBranch,
	}

	if err := s.recycler.Recycle(ctx, sess.Path, s.config.Commands.Recycle, data, w); err != nil {
		return fmt.Errorf("recycle session %s: %w", id, err)
	}

	// Rename directory to recycled pattern immediately
	repoName := git.ExtractRepoName(sess.Remote)
	newPath := filepath.Join(s.config.ReposDir(), fmt.Sprintf("%s-recycle-%s", repoName, generateID()))

	if err := os.Rename(sess.Path, newPath); err != nil {
		return fmt.Errorf("rename session directory: %w", err)
	}

	sess.Path = newPath
	sess.MarkRecycled(time.Now())

	if err := s.sessions.Save(ctx, sess); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	// Enforce max recycled limit
	if err := s.enforceMaxRecycled(ctx, sess.Remote); err != nil {
		s.log.Warn().Err(err).Str("remote", sess.Remote).Msg("failed to enforce max recycled limit")
	}

	s.log.Info().Str("session_id", id).Str("path", newPath).Msg("session recycled")

	return nil
}

// DeleteSession removes a session and its directory.
func (s *Service) DeleteSession(ctx context.Context, id string) error {
	sess, err := s.sessions.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	s.log.Info().Str("session_id", id).Str("path", sess.Path).Msg("deleting session")

	// Remove directory
	if err := os.RemoveAll(sess.Path); err != nil {
		return fmt.Errorf("remove directory: %w", err)
	}

	// Delete from store
	if err := s.sessions.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// Prune removes recycled and corrupted sessions and their directories.
// If all is true, deletes ALL recycled sessions.
// If all is false, respects max_recycled limit per repository (keeps newest N).
func (s *Service) Prune(ctx context.Context, all bool) (int, error) {
	s.log.Info().Bool("all", all).Msg("pruning sessions")

	sessions, err := s.sessions.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list sessions: %w", err)
	}

	count := 0

	// Always delete corrupted sessions
	for _, sess := range sessions {
		if sess.State == session.StateCorrupted {
			if err := s.DeleteSession(ctx, sess.ID); err != nil {
				s.log.Warn().Err(err).Str("session_id", sess.ID).Msg("failed to delete corrupted session")
				continue
			}
			count++
		}
	}

	if all {
		// Delete ALL recycled sessions
		for _, sess := range sessions {
			if sess.State == session.StateRecycled {
				if err := s.DeleteSession(ctx, sess.ID); err != nil {
					s.log.Warn().Err(err).Str("session_id", sess.ID).Msg("failed to prune session")
					continue
				}
				count++
			}
		}
	} else {
		// Respect max_recycled limit per repository
		deleted, err := s.pruneExcessRecycled(ctx, sessions)
		if err != nil {
			return count, fmt.Errorf("prune excess recycled: %w", err)
		}
		count += deleted
	}

	s.log.Info().Int("count", count).Msg("prune complete")

	return count, nil
}

// pruneExcessRecycled deletes recycled sessions exceeding max_recycled per repository.
func (s *Service) pruneExcessRecycled(ctx context.Context, sessions []session.Session) (int, error) {
	// Group recycled sessions by remote
	byRemote := make(map[string][]session.Session)
	for _, sess := range sessions {
		if sess.State == session.StateRecycled {
			byRemote[sess.Remote] = append(byRemote[sess.Remote], sess)
		}
	}

	count := 0
	for remote, recycled := range byRemote {
		limit := s.config.GetMaxRecycled(remote)
		if limit == 0 || len(recycled) <= limit {
			continue
		}

		// Sort by UpdatedAt descending (newest first)
		sort.Slice(recycled, func(i, j int) bool {
			return recycled[i].UpdatedAt.After(recycled[j].UpdatedAt)
		})

		// Delete oldest sessions beyond the limit
		for _, sess := range recycled[limit:] {
			if err := s.DeleteSession(ctx, sess.ID); err != nil {
				s.log.Warn().Err(err).Str("session_id", sess.ID).Msg("failed to delete excess session")
				continue
			}
			count++
		}
	}

	return count, nil
}

// DetectRemote gets the git remote URL from the specified directory.
func (s *Service) DetectRemote(ctx context.Context, dir string) (string, error) {
	return s.git.RemoteURL(ctx, dir)
}

// Git returns the git client for use in background operations.
func (s *Service) Git() git.Git {
	return s.git
}

// generateID creates a 6-character random alphanumeric session ID.
func generateID() string {
	return randid.Generate(6)
}

// findValidRecyclable finds a recyclable session and validates it.
// Returns nil if none found or all candidates are corrupted.
func (s *Service) findValidRecyclable(ctx context.Context, remote string) *session.Session {
	sessions, err := s.sessions.List(ctx)
	if err != nil {
		s.log.Warn().Err(err).Msg("failed to list sessions")
		return nil
	}

	for i := range sessions {
		sess := &sessions[i]

		// Skip non-recyclable sessions
		if sess.State != session.StateRecycled || sess.Remote != remote {
			continue
		}

		// Validate the repository
		if err := s.git.IsValidRepo(ctx, sess.Path); err != nil {
			s.log.Warn().Err(err).Str("session_id", sess.ID).Str("path", sess.Path).Msg("corrupted session found")
			s.markCorrupted(ctx, sess)
			continue
		}

		return sess
	}

	return nil
}

// markCorrupted marks a session as corrupted and optionally deletes it.
func (s *Service) markCorrupted(ctx context.Context, sess *session.Session) {
	sess.MarkCorrupted(time.Now())

	if s.config.AutoDeleteCorrupted {
		s.log.Info().Str("session_id", sess.ID).Msg("auto-deleting corrupted session")
		if err := s.DeleteSession(ctx, sess.ID); err != nil {
			s.log.Warn().Err(err).Str("session_id", sess.ID).Msg("failed to delete corrupted session, marking instead")
			// Fall through to save as corrupted
			if err := s.sessions.Save(ctx, *sess); err != nil {
				s.log.Error().Err(err).Str("session_id", sess.ID).Msg("failed to save corrupted session")
			}
		}
	} else {
		if err := s.sessions.Save(ctx, *sess); err != nil {
			s.log.Error().Err(err).Str("session_id", sess.ID).Msg("failed to save corrupted session")
		}
	}
}

// executeRules executes all rules matching the remote URL.
func (s *Service) executeRules(ctx context.Context, remote, source, dest string) error {
	for _, rule := range s.config.Rules {
		matched, err := matchRemotePattern(rule.Pattern, remote)
		if err != nil {
			return fmt.Errorf("match pattern %q: %w", rule.Pattern, err)
		}
		if !matched {
			continue
		}

		s.log.Debug().
			Str("pattern", rule.Pattern).
			Strs("commands", rule.Commands).
			Strs("copy", rule.Copy).
			Msg("rule matched")

		// Copy files first (so hooks can operate on them)
		if len(rule.Copy) > 0 && source != "" {
			if err := s.fileCopier.CopyFiles(ctx, rule, source, dest); err != nil {
				return fmt.Errorf("copy files: %w", err)
			}
		}

		// Run commands
		if len(rule.Commands) > 0 {
			if err := s.hookRunner.RunHooks(ctx, rule, dest); err != nil {
				return fmt.Errorf("run hooks: %w", err)
			}
		}
	}
	return nil
}

// enforceMaxRecycled deletes oldest recycled sessions for a remote when limit is exceeded.
func (s *Service) enforceMaxRecycled(ctx context.Context, remote string) error {
	limit := s.config.GetMaxRecycled(remote)
	if limit == 0 {
		// Unlimited
		return nil
	}

	sessions, err := s.sessions.List(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	// Collect recycled sessions for this remote
	var recycled []session.Session
	for _, sess := range sessions {
		if sess.State == session.StateRecycled && sess.Remote == remote {
			recycled = append(recycled, sess)
		}
	}

	// Nothing to enforce
	if len(recycled) <= limit {
		return nil
	}

	// Sort by UpdatedAt descending (newest first)
	sort.Slice(recycled, func(i, j int) bool {
		return recycled[i].UpdatedAt.After(recycled[j].UpdatedAt)
	})

	// Delete oldest sessions beyond the limit
	for _, sess := range recycled[limit:] {
		s.log.Info().
			Str("session_id", sess.ID).
			Str("remote", remote).
			Int("limit", limit).
			Msg("deleting excess recycled session")

		if err := s.DeleteSession(ctx, sess.ID); err != nil {
			s.log.Warn().Err(err).Str("session_id", sess.ID).Msg("failed to delete excess recycled session")
		}
	}

	return nil
}
