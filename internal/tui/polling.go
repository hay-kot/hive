package tui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// activitiesLoadedMsg is sent when activities are loaded from the store.
type activitiesLoadedMsg struct {
	activities []messaging.Activity
	err        error
}

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

// loadActivities returns a command that loads activities from the store.
func loadActivities(store messaging.ActivityStore, since time.Time, limit int) tea.Cmd {
	return func() tea.Msg {
		if store == nil {
			return activitiesLoadedMsg{err: nil}
		}

		var activities []messaging.Activity
		var err error
		if since.IsZero() {
			activities, err = store.List(limit)
		} else {
			activities, err = store.ListSince(since, limit)
		}
		if err != nil {
			return activitiesLoadedMsg{err: err}
		}

		return activitiesLoadedMsg{activities: activities, err: nil}
	}
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
