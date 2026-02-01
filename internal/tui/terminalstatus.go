package tui

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/integration/terminal"
)

const terminalStatusTimeout = 2 * time.Second

// TerminalStatus holds the terminal integration status for a session.
type TerminalStatus struct {
	Status    terminal.Status
	Tool      string
	IsLoading bool
	Error     error
}

// terminalStatusBatchCompleteMsg is sent when all terminal status fetches complete.
type terminalStatusBatchCompleteMsg struct {
	Results map[string]TerminalStatus // sessionID -> status
}

// terminalPollTickMsg triggers a terminal status poll cycle.
type terminalPollTickMsg struct{}

// fetchTerminalStatusBatch returns a command that fetches terminal status for multiple sessions.
func fetchTerminalStatusBatch(mgr *terminal.Manager, sessions []*session.Session, workers int) tea.Cmd {
	if mgr == nil || len(sessions) == 0 || !mgr.HasEnabledIntegrations() {
		return nil
	}

	return func() tea.Msg {
		// Refresh integration caches once before fetching statuses
		mgr.RefreshAll()

		results := make(map[string]TerminalStatus)
		var mu sync.Mutex

		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup

		for _, sess := range sessions {
			// Skip non-active sessions
			if sess.State != session.StateActive {
				continue
			}

			wg.Add(1)
			go func(s *session.Session) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				ctx, cancel := context.WithTimeout(context.Background(), terminalStatusTimeout)
				defer cancel()

				status := fetchTerminalStatusForSession(ctx, mgr, s)

				mu.Lock()
				results[s.ID] = status
				mu.Unlock()
			}(sess)
		}

		wg.Wait()
		return terminalStatusBatchCompleteMsg{Results: results}
	}
}

// fetchTerminalStatusForSession fetches terminal status for a single session.
func fetchTerminalStatusForSession(ctx context.Context, mgr *terminal.Manager, sess *session.Session) TerminalStatus {
	status := TerminalStatus{
		Status: terminal.StatusMissing,
	}

	// Try to discover terminal session
	info, integration, err := mgr.DiscoverSession(ctx, sess.Slug, sess.Metadata)
	if err != nil {
		status.Error = err
		return status
	}

	if info == nil || integration == nil {
		return status
	}

	// Get status from integration
	termStatus, err := integration.GetStatus(ctx, info)
	if err != nil {
		status.Error = err
		return status
	}

	status.Status = termStatus
	status.Tool = info.DetectedTool
	return status
}

// startTerminalPollTicker returns a command that starts the terminal status poll ticker.
func startTerminalPollTicker(interval time.Duration) tea.Cmd {
	if interval <= 0 {
		return nil
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return terminalPollTickMsg{}
	})
}
