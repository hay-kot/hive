package tui

import (
	"context"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/hive"
	"github.com/hay-kot/hive/pkg/tmpl"
)

// ActionType identifies the kind of action a keybinding triggers.
type ActionType int

const (
	ActionTypeNone ActionType = iota
	ActionTypeRecycle
	ActionTypeDelete
	ActionTypeShell
)

// Action represents a resolved keybinding action ready for execution.
type Action struct {
	Type        ActionType
	Key         string
	Help        string
	Confirm     string // Non-empty if confirmation required
	ShellCmd    string // For shell actions, the rendered command
	SessionID   string
	SessionPath string
	Silent      bool // Skip loading popup for fast commands
	Exit        bool // Exit hive after command completes
}

// NeedsConfirm returns true if the action requires user confirmation.
func (a Action) NeedsConfirm() bool {
	return a.Confirm != ""
}

// KeybindingHandler resolves keybindings to actions.
type KeybindingHandler struct {
	keybindings map[string]config.Keybinding
	service     *hive.Service
}

// NewKeybindingHandler creates a new handler with the given config.
func NewKeybindingHandler(keybindings map[string]config.Keybinding, service *hive.Service) *KeybindingHandler {
	return &KeybindingHandler{
		keybindings: keybindings,
		service:     service,
	}
}

// Resolve attempts to resolve a key press to an action for the given session.
func (h *KeybindingHandler) Resolve(key string, sess session.Session) (Action, bool) {
	kb, exists := h.keybindings[key]
	if !exists {
		return Action{}, false
	}

	action := Action{
		Key:         key,
		Help:        kb.Help,
		Confirm:     kb.Confirm,
		SessionID:   sess.ID,
		SessionPath: sess.Path,
		Silent:      kb.Silent,
		Exit:        kb.Exit,
	}

	// Built-in actions
	if kb.Action != "" {
		switch kb.Action {
		case config.ActionRecycle:
			action.Type = ActionTypeRecycle
			if action.Help == "" {
				action.Help = "recycle"
			}
		case config.ActionDelete:
			action.Type = ActionTypeDelete
			if action.Help == "" {
				action.Help = "delete"
			}
		}
		return action, true
	}

	// Shell command
	if kb.Sh != "" {
		data := struct {
			Path   string
			Remote string
			ID     string
			Name   string
		}{
			Path:   sess.Path,
			Remote: sess.Remote,
			ID:     sess.ID,
			Name:   sess.Name,
		}

		rendered, err := tmpl.Render(kb.Sh, data)
		if err != nil {
			// Template error - return action with empty command
			action.Type = ActionTypeShell
			action.ShellCmd = fmt.Sprintf("echo 'template error: %v'", err)
			return action, true
		}

		action.Type = ActionTypeShell
		action.ShellCmd = rendered
		return action, true
	}

	return Action{}, false
}

// Execute runs the given action.
// Note: ActionTypeRecycle is not handled here - it uses streaming output
// and is executed directly by the TUI model via Service.RecycleSession.
func (h *KeybindingHandler) Execute(ctx context.Context, action Action) error {
	switch action.Type {
	case ActionTypeDelete:
		return h.service.DeleteSession(ctx, action.SessionID)
	case ActionTypeShell:
		return h.executeShell(ctx, action.ShellCmd)
	default:
		return fmt.Errorf("action type %d not supported by Execute", action.Type)
	}
}

// executeShell runs a shell command.
func (h *KeybindingHandler) executeShell(_ context.Context, cmd string) error {
	// Use sh -c to execute the command
	c := exec.Command("sh", "-c", cmd)
	return c.Run()
}

// HelpEntries returns all configured keybindings for display, sorted by key.
func (h *KeybindingHandler) HelpEntries() []string {
	// Get sorted keys for consistent ordering
	keys := slices.Sorted(maps.Keys(h.keybindings))

	entries := make([]string, 0, len(h.keybindings))
	for _, key := range keys {
		kb := h.keybindings[key]
		help := kb.Help
		if help == "" && kb.Action != "" {
			help = kb.Action
		}
		if help == "" {
			help = "shell"
		}
		entries = append(entries, fmt.Sprintf("[%s] %s", key, help))
	}
	return entries
}

// HelpString returns a formatted help string for all keybindings.
func (h *KeybindingHandler) HelpString() string {
	entries := h.HelpEntries()
	return strings.Join(entries, "  ")
}

// KeyBindings returns key.Binding objects for integration with bubbles help system.
func (h *KeybindingHandler) KeyBindings() []key.Binding {
	keys := slices.Sorted(maps.Keys(h.keybindings))
	bindings := make([]key.Binding, 0, len(keys))

	for _, k := range keys {
		kb := h.keybindings[k]
		help := kb.Help
		if help == "" && kb.Action != "" {
			help = kb.Action
		}
		if help == "" {
			help = "shell"
		}

		bindings = append(bindings, key.NewBinding(
			key.WithKeys(k),
			key.WithHelp(k, help),
		))
	}

	return bindings
}
