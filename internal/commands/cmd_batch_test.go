package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBatchInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   BatchInput
		wantErr string
	}{
		{
			name:    "empty sessions",
			input:   BatchInput{Sessions: []BatchSession{}},
			wantErr: "sessions",
		},
		{
			name: "missing name",
			input: BatchInput{Sessions: []BatchSession{
				{Prompt: "do something"},
			}},
			wantErr: "name",
		},
		{
			name: "whitespace name",
			input: BatchInput{Sessions: []BatchSession{
				{Name: "   "},
			}},
			wantErr: "name",
		},
		{
			name: "duplicate names",
			input: BatchInput{Sessions: []BatchSession{
				{Name: "test"},
				{Name: "test"},
			}},
			wantErr: "duplicate",
		},
		{
			name: "valid input",
			input: BatchInput{Sessions: []BatchSession{
				{Name: "session1", Prompt: "prompt1"},
				{Name: "session2", Remote: "https://github.com/org/repo"},
			}},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestBatchInput_JSON(t *testing.T) {
	jsonInput := `{
		"sessions": [
			{"name": "task1", "prompt": "Do task 1"},
			{"name": "task2", "remote": "https://github.com/org/repo", "prompt": "Do task 2"}
		]
	}`

	var input BatchInput
	if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(input.Sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(input.Sessions))
	}

	if input.Sessions[0].Name != "task1" {
		t.Errorf("expected name 'task1', got %q", input.Sessions[0].Name)
	}

	if input.Sessions[1].Remote != "https://github.com/org/repo" {
		t.Errorf("expected remote URL, got %q", input.Sessions[1].Remote)
	}
}

func TestBatchOutput_JSON(t *testing.T) {
	output := BatchOutput{
		BatchID: "abc123",
		LogFile: "/tmp/logs/batch-abc123.log",
		Results: []BatchResult{
			{Name: "task1", SessionID: "def456", Path: "/tmp/session", Status: StatusCreated},
			{Name: "task2", Status: StatusFailed, Error: "clone failed"},
			{Name: "task3", Status: StatusSkipped},
		},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BatchOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.BatchID != "abc123" {
		t.Errorf("expected batch_id 'abc123', got %q", decoded.BatchID)
	}

	if decoded.LogFile != "/tmp/logs/batch-abc123.log" {
		t.Errorf("expected log_file path, got %q", decoded.LogFile)
	}

	if len(decoded.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(decoded.Results))
	}

	if decoded.Results[0].Status != StatusCreated {
		t.Errorf("expected status 'created', got %q", decoded.Results[0].Status)
	}

	if decoded.Results[1].Error != "clone failed" {
		t.Errorf("expected error message, got %q", decoded.Results[1].Error)
	}

	if decoded.Results[2].Status != StatusSkipped {
		t.Errorf("expected status 'skipped', got %q", decoded.Results[2].Status)
	}
}

func TestBatchErrorOutput_JSON(t *testing.T) {
	output := BatchErrorOutput{Error: "something went wrong"}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded BatchErrorOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Error != "something went wrong" {
		t.Errorf("expected error message, got %q", decoded.Error)
	}
}

func TestCountByStatus(t *testing.T) {
	results := []BatchResult{
		{Status: StatusCreated},
		{Status: StatusCreated},
		{Status: StatusFailed},
		{Status: StatusSkipped},
		{Status: StatusSkipped},
		{Status: StatusSkipped},
	}

	if got := countByStatus(results, StatusCreated); got != 2 {
		t.Errorf("countByStatus(created) = %d, want 2", got)
	}
	if got := countByStatus(results, StatusFailed); got != 1 {
		t.Errorf("countByStatus(failed) = %d, want 1", got)
	}
	if got := countByStatus(results, StatusSkipped); got != 3 {
		t.Errorf("countByStatus(skipped) = %d, want 3", got)
	}
}
