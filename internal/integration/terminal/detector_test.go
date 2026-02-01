package terminal

import (
	"testing"
)

func TestDetector_IsBusy(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		content string
		want    bool
	}{
		{
			name:    "ctrl+c to interrupt",
			tool:    "claude",
			content: "Thinking... (45s · 1234 tokens · ctrl+c to interrupt)",
			want:    true,
		},
		{
			name:    "esc to interrupt",
			tool:    "claude",
			content: "Working... (esc to interrupt)",
			want:    true,
		},
		{
			name:    "spinner character",
			tool:    "claude",
			content: "Some output\n⠙ Processing...",
			want:    true,
		},
		{
			name:    "thinking with tokens",
			tool:    "claude",
			content: "Thinking about your request (12 tokens used)",
			want:    true,
		},
		{
			name:    "idle prompt",
			tool:    "claude",
			content: "Task completed.\n❯",
			want:    false,
		},
		{
			name:    "empty content",
			tool:    "claude",
			content: "",
			want:    false,
		},
		{
			name:    "asterisk spinner",
			tool:    "claude",
			content: "Some output\n✳ Processing...",
			want:    true,
		},
		{
			name:    "whimsical word with ellipsis",
			tool:    "claude",
			content: "Some output\n⠙ pondering…",
			want:    true,
		},
		{
			name:    "whimsical word clauding",
			tool:    "claude",
			content: "Some output\nclauding...",
			want:    true,
		},
		{
			name:    "connecting with tokens",
			tool:    "claude",
			content: "Connecting to API (42 tokens used)",
			want:    true,
		},
		{
			name:    "box drawing line - not busy",
			tool:    "claude",
			content: "│ Some permission dialog\n│ ⠙ not a spinner",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDetector(tt.tool)
			if got := d.IsBusy(tt.content); got != tt.want {
				t.Errorf("IsBusy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_NeedsApproval(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		content string
		want    bool
	}{
		{
			name:    "permission prompt - allow once",
			tool:    "claude",
			content: "Do you want to run this command?\nYes, allow once\nYes, allow always",
			want:    true,
		},
		{
			name:    "permission prompt - tell differently",
			tool:    "claude",
			content: "Execute this?\nNo, and tell Claude what to do differently",
			want:    true,
		},
		{
			name:    "Y/n prompt",
			tool:    "claude",
			content: "Do you want to continue? (Y/n)",
			want:    true,
		},
		{
			name:    "arrow keys navigation",
			tool:    "claude",
			content: "Select an option:\nUse arrow keys to navigate",
			want:    true,
		},
		{
			name:    "box drawing permission dialog",
			tool:    "claude",
			content: "╭─ Permission ─╮\n│ Do you want to run this?\n│ Yes / No",
			want:    true,
		},
		{
			name:    "selection indicator",
			tool:    "claude",
			content: "Choose an option:\n❯ Yes\n  No",
			want:    true,
		},
		{
			name:    "plan approval prompt",
			tool:    "claude",
			content: "Here is the plan:\n1. Step one\nApprove this plan?",
			want:    true,
		},
		{
			name:    "busy - not approval",
			tool:    "claude",
			content: "⠙ Working... ctrl+c to interrupt",
			want:    false,
		},
		{
			name:    "standalone prompt - not approval",
			tool:    "claude",
			content: "Previous output\n❯",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDetector(tt.tool)
			if got := d.NeedsApproval(tt.content); got != tt.want {
				t.Errorf("NeedsApproval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_IsReady(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		content string
		want    bool
	}{
		{
			name:    "standalone prompt character",
			tool:    "claude",
			content: "Previous output\n❯",
			want:    true,
		},
		{
			name:    "standalone > prompt",
			tool:    "claude",
			content: "Previous output\n>",
			want:    true,
		},
		{
			name:    "prompt with suggestion",
			tool:    "claude",
			content: "Done!\n❯ Try asking about tests",
			want:    true,
		},
		{
			name:    "non-breaking space in prompt",
			tool:    "claude",
			content: "Done.\n❯\u00A0",
			want:    true,
		},
		{
			name:    "busy - not ready",
			tool:    "claude",
			content: "⠙ Working... ctrl+c to interrupt",
			want:    false,
		},
		{
			name:    "approval dialog - not ready",
			tool:    "claude",
			content: "Yes, allow once\nYes, allow always",
			want:    false,
		},
		{
			name:    "regular output - not ready",
			tool:    "claude",
			content: "Here is the code:\nfunction hello() { }",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDetector(tt.tool)
			if got := d.IsReady(tt.content); got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_DetectStatus(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		content string
		want    Status
	}{
		{
			name:    "active - spinner",
			tool:    "claude",
			content: "⠙ Processing your request...",
			want:    StatusActive,
		},
		{
			name:    "active - interrupt indicator",
			tool:    "claude",
			content: "Thinking... (ctrl+c to interrupt)",
			want:    StatusActive,
		},
		{
			name:    "approval - permission",
			tool:    "claude",
			content: "Yes, allow once\nYes, allow always",
			want:    StatusApproval,
		},
		{
			name:    "ready - prompt",
			tool:    "claude",
			content: "Done.\n❯",
			want:    StatusReady,
		},
		{
			name:    "ready - regular output defaults to ready",
			tool:    "claude",
			content: "Here is the result:\nfunction foo() {}",
			want:    StatusReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDetector(tt.tool)
			if got := d.DetectStatus(tt.content); got != tt.want {
				t.Errorf("DetectStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectTool(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "claude keyword",
			content: "Welcome to Claude Code!",
			want:    "claude",
		},
		{
			name:    "anthropic keyword",
			content: "Powered by Anthropic",
			want:    "claude",
		},
		{
			name:    "ctrl+c to interrupt (claude specific)",
			content: "Thinking... ctrl+c to interrupt",
			want:    "claude",
		},
		{
			name:    "gemini keyword",
			content: "Welcome to Gemini CLI",
			want:    "gemini",
		},
		{
			name:    "opencode keyword",
			content: "OpenCode v1.0",
			want:    "opencode",
		},
		{
			name:    "codex keyword",
			content: "OpenAI Codex",
			want:    "codex",
		},
		{
			name:    "unknown - defaults to shell",
			content: "user@host:~$",
			want:    "shell",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectTool(tt.content); got != tt.want {
				t.Errorf("DetectTool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "no ansi",
			content: "plain text",
			want:    "plain text",
		},
		{
			name:    "color code",
			content: "\x1b[32mgreen\x1b[0m",
			want:    "green",
		},
		{
			name:    "multiple codes",
			content: "\x1b[1m\x1b[31mbold red\x1b[0m normal",
			want:    "bold red normal",
		},
		{
			name:    "cursor movement",
			content: "\x1b[2Amove up",
			want:    "move up",
		},
		{
			name:    "osc sequence with bell",
			content: "before\x1b]0;title\x07after",
			want:    "beforeafter",
		},
		{
			name:    "empty string",
			content: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripANSI(tt.content); got != tt.want {
				t.Errorf("stripANSI() = %v, want %v", got, tt.want)
			}
		})
	}
}
