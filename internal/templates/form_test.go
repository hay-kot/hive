package templates

import (
	"reflect"
	"testing"

	"github.com/hay-kot/hive/internal/core/config"
)

func TestParseSetValues(t *testing.T) {
	tests := []struct {
		name    string
		sets    []string
		want    map[string]any
		wantErr string
	}{
		{
			name: "single value",
			sets: []string{"name=value"},
			want: map[string]any{"name": "value"},
		},
		{
			name: "multiple single values",
			sets: []string{"name=value", "other=test"},
			want: map[string]any{"name": "value", "other": "test"},
		},
		{
			name: "multi-value with commas",
			sets: []string{"tags=a,b,c"},
			want: map[string]any{"tags": []string{"a", "b", "c"}},
		},
		{
			name: "mixed single and multi values",
			sets: []string{"name=value", "tags=a,b"},
			want: map[string]any{"name": "value", "tags": []string{"a", "b"}},
		},
		{
			name: "value with equals sign",
			sets: []string{"expr=a=b"},
			want: map[string]any{"expr": "a=b"},
		},
		{
			name: "empty value",
			sets: []string{"name="},
			want: map[string]any{"name": ""},
		},
		{
			name:    "missing equals",
			sets:    []string{"namevalue"},
			wantErr: "invalid --set format",
		},
		{
			name:    "empty name",
			sets:    []string{"=value"},
			wantErr: "empty name",
		},
		{
			name: "whitespace in multi values",
			sets: []string{"tags=a, b, c"},
			want: map[string]any{"tags": []string{"a", "b", "c"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSetValues(tt.sets)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("ParseSetValues() expected error containing %q, got nil", tt.wantErr)
					return
				}
				if !containsString(err.Error(), tt.wantErr) {
					t.Errorf("ParseSetValues() error = %q, want error containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSetValues() unexpected error: %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSetValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    config.Template
		values  map[string]any
		wantErr string
	}{
		{
			name: "all required fields present",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "name", Type: config.FieldTypeString, Required: true},
					{Name: "desc", Type: config.FieldTypeText, Required: true},
				},
			},
			values:  map[string]any{"name": "value", "desc": "description"},
			wantErr: "",
		},
		{
			name: "optional fields can be missing",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "name", Type: config.FieldTypeString, Required: true},
					{Name: "optional", Type: config.FieldTypeString, Required: false},
				},
			},
			values:  map[string]any{"name": "value"},
			wantErr: "",
		},
		{
			name: "missing required field",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "name", Type: config.FieldTypeString, Required: true},
				},
			},
			values:  map[string]any{},
			wantErr: "required field \"name\" is missing",
		},
		{
			name: "empty required string field",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "name", Type: config.FieldTypeString, Required: true},
				},
			},
			values:  map[string]any{"name": ""},
			wantErr: "required field \"name\" is empty",
		},
		{
			name: "whitespace-only required field",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "name", Type: config.FieldTypeString, Required: true},
				},
			},
			values:  map[string]any{"name": "   "},
			wantErr: "required field \"name\" is empty",
		},
		{
			name: "empty multi-select required field",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "tags", Type: config.FieldTypeMultiSelect, Required: true, Options: []config.FieldOption{{Value: "a"}}},
				},
			},
			values:  map[string]any{"tags": []string{}},
			wantErr: "required field \"tags\" has no selections",
		},
		{
			name: "valid multi-select required field",
			tmpl: config.Template{
				Prompt: "test",
				Fields: []config.TemplateField{
					{Name: "tags", Type: config.FieldTypeMultiSelect, Required: true, Options: []config.FieldOption{{Value: "a"}}},
				},
			},
			values:  map[string]any{"tags": []string{"a"}},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequiredFields(tt.tmpl, tt.values)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateRequiredFields() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("ValidateRequiredFields() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("ValidateRequiredFields() error = %q, want error containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestFieldTitle(t *testing.T) {
	tests := []struct {
		name  string
		field config.TemplateField
		want  string
	}{
		{
			name:  "label only",
			field: config.TemplateField{Label: "My Label", Name: "name"},
			want:  "My Label",
		},
		{
			name:  "name fallback",
			field: config.TemplateField{Name: "myName"},
			want:  "myName",
		},
		{
			name:  "required indicator",
			field: config.TemplateField{Label: "My Label", Name: "name", Required: true},
			want:  "My Label *",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fieldTitle(tt.field); got != tt.want {
				t.Errorf("fieldTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractValue(t *testing.T) {
	str := "test"
	slice := []string{"a", "b"}

	tests := []struct {
		name    string
		binding any
		want    any
	}{
		{
			name:    "string pointer",
			binding: &str,
			want:    "test",
		},
		{
			name:    "slice pointer",
			binding: &slice,
			want:    []string{"a", "b"},
		},
		{
			name:    "unknown type",
			binding: 123,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractValue(tt.binding)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
