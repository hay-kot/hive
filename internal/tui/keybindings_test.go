package tui

import (
	"testing"

	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/session"
)

func TestKeybindingHandler_Resolve_RecycledSession(t *testing.T) {
	keybindings := map[string]config.Keybinding{
		"d": {Action: config.ActionDelete, Help: "delete"},
		"r": {Action: config.ActionRecycle, Help: "recycle"},
		"o": {Sh: "code {{ .Path }}", Help: "open in vscode"},
	}

	handler := NewKeybindingHandler(keybindings, nil)

	activeSession := session.Session{
		ID:    "test-id",
		Path:  "/test/path",
		State: session.StateActive,
	}

	recycledSession := session.Session{
		ID:    "test-id",
		Path:  "/test/path",
		State: session.StateRecycled,
	}

	tests := []struct {
		name    string
		key     string
		sess    session.Session
		wantOK  bool
		wantTyp ActionType
	}{
		{
			name:    "active session allows delete",
			key:     "d",
			sess:    activeSession,
			wantOK:  true,
			wantTyp: ActionTypeDelete,
		},
		{
			name:    "active session allows recycle",
			key:     "r",
			sess:    activeSession,
			wantOK:  true,
			wantTyp: ActionTypeRecycle,
		},
		{
			name:    "active session allows shell command",
			key:     "o",
			sess:    activeSession,
			wantOK:  true,
			wantTyp: ActionTypeShell,
		},
		{
			name:    "recycled session allows delete",
			key:     "d",
			sess:    recycledSession,
			wantOK:  true,
			wantTyp: ActionTypeDelete,
		},
		{
			name:   "recycled session blocks recycle",
			key:    "r",
			sess:   recycledSession,
			wantOK: false,
		},
		{
			name:   "recycled session blocks shell command",
			key:    "o",
			sess:   recycledSession,
			wantOK: false,
		},
		{
			name:   "unknown key returns false",
			key:    "x",
			sess:   activeSession,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, ok := handler.Resolve(tt.key, tt.sess)
			if ok != tt.wantOK {
				t.Errorf("Resolve() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && action.Type != tt.wantTyp {
				t.Errorf("Resolve() action.Type = %v, want %v", action.Type, tt.wantTyp)
			}
		})
	}
}
