package tui

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/hay-kot/hive/internal/core/messaging"
)

const messagesPollInterval = 500 * time.Millisecond

// messagesLoadedMsg is sent when messages are loaded from the store.
type messagesLoadedMsg struct {
	messages []messaging.Message
	err      error
}

// pollTickMsg is sent to trigger the next poll.
type pollTickMsg struct{}

// sessionRefreshTickMsg is sent to trigger session list refresh.
type sessionRefreshTickMsg struct{}

// loadMessages returns a command that loads messages from the store.
func loadMessages(store messaging.Store, topic string, since time.Time) tea.Cmd {
	return func() tea.Msg {
		if store == nil {
			return messagesLoadedMsg{err: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		messages, err := store.Subscribe(ctx, topic, since)
		if err != nil {
			// ErrTopicNotFound is not an error, just no messages
			if errors.Is(err, messaging.ErrTopicNotFound) {
				return messagesLoadedMsg{messages: nil, err: nil}
			}
			return messagesLoadedMsg{err: err}
		}

		return messagesLoadedMsg{messages: messages, err: nil}
	}
}

// schedulePollTick returns a command that schedules the next poll tick.
func schedulePollTick() tea.Cmd {
	return tea.Tick(messagesPollInterval, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

// scheduleSessionRefresh returns a command that schedules the next session refresh.
func (m Model) scheduleSessionRefresh() tea.Cmd {
	interval := m.cfg.TUI.RefreshInterval
	if interval == 0 {
		return nil // Disabled
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return sessionRefreshTickMsg{}
	})
}

// Animation constants.
const animationTickInterval = 100 * time.Millisecond

// animationTickMsg is sent to advance the status animation.
type animationTickMsg struct{}

// scheduleAnimationTick returns a command that schedules the next animation frame.
func scheduleAnimationTick() tea.Cmd {
	return tea.Tick(animationTickInterval, func(time.Time) tea.Msg {
		return animationTickMsg{}
	})
}
