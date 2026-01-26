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
