package templates

import (
	"testing"

	"github.com/hay-kot/hive/internal/core/config"
)

func TestRenderPrompt(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    config.Template
		values  map[string]any
		want    string
		wantErr string
	}{
		{
			name: "simple string substitution",
			tmpl: config.Template{
				Prompt: "Hello {{ .name }}",
				Fields: []config.TemplateField{
					{Name: "name", Type: config.FieldTypeString},
				},
			},
			values: map[string]any{"name": "World"},
			want:   "Hello World",
		},
		{
			name: "multi-select with join",
			tmpl: config.Template{
				Prompt: "Focus on: {{ .areas | join \", \" }}",
				Fields: []config.TemplateField{
					{Name: "areas", Type: config.FieldTypeMultiSelect, Options: []config.FieldOption{{Value: "a"}, {Value: "b"}}},
				},
			},
			values: map[string]any{"areas": []string{"security", "performance"}},
			want:   "Focus on: security, performance",
		},
		{
			name: "default value for missing field",
			tmpl: config.Template{
				Prompt: "Value: {{ .opt | default \"none\" }}",
				Fields: []config.TemplateField{
					{Name: "opt", Type: config.FieldTypeString},
				},
			},
			values: map[string]any{},
			want:   "Value: none",
		},
		{
			name: "field default used when not provided",
			tmpl: config.Template{
				Prompt: "Priority: {{ .priority }}",
				Fields: []config.TemplateField{
					{Name: "priority", Type: config.FieldTypeString, Default: "normal"},
				},
			},
			values: map[string]any{},
			want:   "Priority: normal",
		},
		{
			name: "conditional with if",
			tmpl: config.Template{
				Prompt: "PR #{{ .pr }}{{ if .context }}\nContext: {{ .context }}{{ end }}",
				Fields: []config.TemplateField{
					{Name: "pr", Type: config.FieldTypeString},
					{Name: "context", Type: config.FieldTypeText},
				},
			},
			values: map[string]any{"pr": "123", "context": "bug fix"},
			want:   "PR #123\nContext: bug fix",
		},
		{
			name: "conditional without optional value",
			tmpl: config.Template{
				Prompt: "PR #{{ .pr }}{{ if .context }}\nContext: {{ .context }}{{ end }}",
				Fields: []config.TemplateField{
					{Name: "pr", Type: config.FieldTypeString},
					{Name: "context", Type: config.FieldTypeText},
				},
			},
			values: map[string]any{"pr": "123"},
			want:   "PR #123",
		},
		{
			name: "shell quote function",
			tmpl: config.Template{
				Prompt: "Run: echo {{ .msg | shq }}",
				Fields: []config.TemplateField{
					{Name: "msg", Type: config.FieldTypeString},
				},
			},
			values: map[string]any{"msg": "hello world"},
			want:   "Run: echo 'hello world'",
		},
		{
			name: "shell quote with single quotes",
			tmpl: config.Template{
				Prompt: "echo {{ .msg | shq }}",
				Fields: []config.TemplateField{
					{Name: "msg", Type: config.FieldTypeString},
				},
			},
			values: map[string]any{"msg": "it's working"},
			want:   "echo 'it'\\''s working'",
		},
		{
			name: "empty multi-select",
			tmpl: config.Template{
				Prompt: "{{ if .tags }}Tags: {{ .tags | join \", \" }}{{ else }}No tags{{ end }}",
				Fields: []config.TemplateField{
					{Name: "tags", Type: config.FieldTypeMultiSelect, Options: []config.FieldOption{{Value: "a"}}},
				},
			},
			values: map[string]any{},
			want:   "No tags",
		},
		{
			name:    "empty prompt",
			tmpl:    config.Template{},
			values:  map[string]any{},
			wantErr: "template has no prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderPrompt(tt.tmpl, tt.values)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("RenderPrompt() expected error containing %q, got nil", tt.wantErr)
					return
				}
				if !containsString(err.Error(), tt.wantErr) {
					t.Errorf("RenderPrompt() error = %q, want error containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("RenderPrompt() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("RenderPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderName(t *testing.T) {
	tests := []struct {
		name   string
		tmpl   config.Template
		values map[string]any
		want   string
	}{
		{
			name: "simple name template",
			tmpl: config.Template{
				Name:   "pr-{{ .pr_number }}",
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "pr_number", Type: config.FieldTypeString},
				},
			},
			values: map[string]any{"pr_number": "123"},
			want:   "pr-123",
		},
		{
			name: "no name template",
			tmpl: config.Template{
				Prompt: "test",
			},
			values: map[string]any{},
			want:   "",
		},
		{
			name: "name with multiple fields",
			tmpl: config.Template{
				Name:   "{{ .type }}-{{ .id }}",
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "type", Type: config.FieldTypeString},
					{Name: "id", Type: config.FieldTypeString},
				},
			},
			values: map[string]any{"type": "feature", "id": "42"},
			want:   "feature-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderName(tt.tmpl, tt.values)
			if err != nil {
				t.Errorf("RenderName() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("RenderName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildTemplateData(t *testing.T) {
	tests := []struct {
		name   string
		tmpl   config.Template
		values map[string]any
		check  func(t *testing.T, data map[string]any)
	}{
		{
			name: "missing string field gets empty string",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "foo", Type: config.FieldTypeString},
				},
			},
			values: map[string]any{},
			check: func(t *testing.T, data map[string]any) {
				if data["foo"] != "" {
					t.Errorf("expected empty string, got %v", data["foo"])
				}
			},
		},
		{
			name: "missing multi-select gets empty slice",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "tags", Type: config.FieldTypeMultiSelect, Options: []config.FieldOption{{Value: "a"}}},
				},
			},
			values: map[string]any{},
			check: func(t *testing.T, data map[string]any) {
				if tags, ok := data["tags"].([]string); !ok || len(tags) != 0 {
					t.Errorf("expected empty []string, got %v", data["tags"])
				}
			},
		},
		{
			name: "default value used when not provided",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "level", Type: config.FieldTypeString, Default: "medium"},
				},
			},
			values: map[string]any{},
			check: func(t *testing.T, data map[string]any) {
				if data["level"] != "medium" {
					t.Errorf("expected 'medium', got %v", data["level"])
				}
			},
		},
		{
			name: "provided value overrides default",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "level", Type: config.FieldTypeString, Default: "medium"},
				},
			},
			values: map[string]any{"level": "high"},
			check: func(t *testing.T, data map[string]any) {
				if data["level"] != "high" {
					t.Errorf("expected 'high', got %v", data["level"])
				}
			},
		},
		{
			name: "extra values passed through",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{},
			},
			values: map[string]any{"extra": "value"},
			check: func(t *testing.T, data map[string]any) {
				if data["extra"] != "value" {
					t.Errorf("expected 'value', got %v", data["extra"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildTemplateData(tt.tmpl, tt.values)
			tt.check(t, data)
		})
	}
}
