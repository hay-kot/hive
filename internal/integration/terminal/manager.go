package terminal

import (
	"context"
	"slices"
	"sync"
)

// Manager manages terminal integrations.
type Manager struct {
	integrations map[string]Integration
	enabled      []string
	mu           sync.RWMutex
}

// NewManager creates a new integration manager with the given enabled integrations.
func NewManager(enabled []string) *Manager {
	return &Manager{
		integrations: make(map[string]Integration),
		enabled:      enabled,
	}
}

// Register adds an integration to the manager.
func (m *Manager) Register(i Integration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.integrations[i.Name()] = i
}

// Get returns an integration by name, or nil if not found.
func (m *Manager) Get(name string) Integration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.integrations[name]
}

// IsEnabled returns true if the named integration is in the enabled list.
func (m *Manager) IsEnabled(name string) bool {
	return slices.Contains(m.enabled, name)
}

// EnabledIntegrations returns all enabled and available integrations.
func (m *Manager) EnabledIntegrations() []Integration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Integration
	for _, name := range m.enabled {
		if i, ok := m.integrations[name]; ok && i.Available() {
			result = append(result, i)
		}
	}
	return result
}

// RefreshAll calls RefreshCache on all enabled integrations.
func (m *Manager) RefreshAll() {
	for _, i := range m.EnabledIntegrations() {
		i.RefreshCache()
	}
}

// DiscoverSession tries all enabled integrations to find a session.
func (m *Manager) DiscoverSession(ctx context.Context, slug string, metadata map[string]string) (*SessionInfo, Integration, error) {
	for _, i := range m.EnabledIntegrations() {
		info, err := i.DiscoverSession(ctx, slug, metadata)
		if err != nil {
			continue
		}
		if info != nil {
			return info, i, nil
		}
	}
	return nil, nil, nil
}

// HasEnabledIntegrations returns true if any integrations are enabled and available.
func (m *Manager) HasEnabledIntegrations() bool {
	return len(m.EnabledIntegrations()) > 0
}
