package templates

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/hay-kot/hive/internal/core/config"
)

// templateFuncs provides template functions for prompt rendering.
var templateFuncs = template.FuncMap{
	// join concatenates slice elements with a separator
	"join": func(sep string, v any) string {
		switch val := v.(type) {
		case []string:
			return strings.Join(val, sep)
		case string:
			return val
		default:
			return ""
		}
	},
	// default returns the default value if the input is empty
	"default": func(def string, v any) string {
		switch val := v.(type) {
		case string:
			if val == "" {
				return def
			}
			return val
		case []string:
			if len(val) == 0 {
				return def
			}
			return strings.Join(val, ", ")
		default:
			if v == nil {
				return def
			}
			return def
		}
	},
	// shq shell-quotes a string for safe use in shell commands
	"shq": func(s string) string {
		if s == "" {
			return "''"
		}
		escaped := strings.ReplaceAll(s, "'", `'\''`)
		return "'" + escaped + "'"
	},
}

// RenderPrompt renders a template's prompt with the given field values.
func RenderPrompt(tmpl config.Template, values map[string]any) (string, error) {
	if tmpl.Prompt == "" {
		return "", fmt.Errorf("template has no prompt")
	}

	// Build data map with default values for missing optional fields
	data := buildTemplateData(tmpl, values)

	return renderTemplate("prompt", tmpl.Prompt, data)
}

// RenderName renders a template's session name with the given field values.
// Returns empty string if no name template is defined.
func RenderName(tmpl config.Template, values map[string]any) (string, error) {
	if tmpl.Name == "" {
		return "", nil
	}

	data := buildTemplateData(tmpl, values)

	return renderTemplate("name", tmpl.Name, data)
}

// buildTemplateData creates the data map for template rendering,
// filling in default values for missing optional fields.
func buildTemplateData(tmpl config.Template, values map[string]any) map[string]any {
	data := make(map[string]any)

	// Initialize all fields with defaults or zero values
	for _, field := range tmpl.Fields {
		if v, ok := values[field.Name]; ok {
			data[field.Name] = v
		} else if field.Default != "" {
			data[field.Name] = field.Default
		} else {
			// Set zero value based on type
			switch field.Type {
			case config.FieldTypeMultiSelect:
				data[field.Name] = []string{}
			default:
				data[field.Name] = ""
			}
		}
	}

	// Add any extra values not defined in fields (allows pass-through)
	for k, v := range values {
		if _, exists := data[k]; !exists {
			data[k] = v
		}
	}

	return data
}

// renderTemplate executes a template string with the given data.
func renderTemplate(name, tmplStr string, data map[string]any) (string, error) {
	// Use missingkey=zero to allow undefined fields to be empty
	t, err := template.New(name).Funcs(templateFuncs).Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}
