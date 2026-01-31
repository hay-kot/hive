package validate

import (
	"testing"
)

func TestSessionName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "my-session", false},
		{"valid with spaces", "my session", false},
		{"empty string", "", true},
		{"only spaces", "   ", true},
		{"only tabs", "\t\t", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SessionName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SessionName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestSessionID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid alphanumeric", "abc123", false},
		{"valid letters only", "abcdef", false},
		{"valid numbers only", "123456", false},
		{"empty string", "", true},
		{"with spaces", "abc 123", true},
		{"with hyphen", "abc-123", true},
		{"with underscore", "abc_123", true},
		{"uppercase letters", "ABC123", true},
		{"mixed case", "AbC123", true},
		{"special chars", "abc!@#", true},
		{"unicode", "abc日本", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SessionID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SessionID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
