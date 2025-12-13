// Package templates handles session template form generation and prompt rendering.
package templates

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/styles"
)

// FormResult holds the collected values from a template form.
type FormResult struct {
	Values map[string]any
}

// AllFieldsPrefilled returns true if all template fields have values in prefilled.
func AllFieldsPrefilled(tmpl config.Template, prefilled map[string]any) bool {
	for _, field := range tmpl.Fields {
		if _, ok := prefilled[field.Name]; !ok {
			return false
		}
	}
	return true
}

// RunForm generates and runs a huh form for the given template, returning collected values.
// If prefilled values are provided, they are used as defaults for the form fields.
// The form is skipped entirely if all fields have prefilled values (use AllFieldsPrefilled to check).
func RunForm(tmpl config.Template, prefilled map[string]any) (*FormResult, error) {
	if prefilled == nil {
		prefilled = make(map[string]any)
	}

	if len(tmpl.Fields) == 0 {
		return &FormResult{Values: prefilled}, nil
	}

	result := &FormResult{Values: make(map[string]any)}
	fields := make([]huh.Field, 0, len(tmpl.Fields))
	bindings := make(map[string]any)

	for _, field := range tmpl.Fields {
		// Get prefilled value if it exists
		prefilledVal, hasPrefilled := prefilled[field.Name]
		f, binding := createFieldWithValue(field, prefilledVal, hasPrefilled)
		if f != nil {
			fields = append(fields, f)
			bindings[field.Name] = binding
		}
	}

	if len(fields) == 0 {
		return result, nil
	}

	form := huh.NewForm(huh.NewGroup(fields...)).WithTheme(styles.FormTheme())

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Extract values from bindings
	for name, binding := range bindings {
		result.Values[name] = extractValue(binding)
	}

	return result, nil
}

// createFieldWithValue creates a huh field from a template field definition with an optional prefilled value.
// Returns the field and a binding pointer for value extraction.
func createFieldWithValue(field config.TemplateField, prefilledVal any, hasPrefilled bool) (huh.Field, any) {
	switch field.Type {
	case config.FieldTypeString:
		return createStringField(field, prefilledVal, hasPrefilled)
	case config.FieldTypeText:
		return createTextField(field, prefilledVal, hasPrefilled)
	case config.FieldTypeSelect:
		return createSelectField(field, prefilledVal, hasPrefilled)
	case config.FieldTypeMultiSelect:
		return createMultiSelectField(field, prefilledVal, hasPrefilled)
	default:
		return nil, nil
	}
}

func createStringField(field config.TemplateField, prefilledVal any, hasPrefilled bool) (huh.Field, any) {
	var value string
	if hasPrefilled {
		if s, ok := prefilledVal.(string); ok {
			value = s
		}
	} else if field.Default != "" {
		value = field.Default
	}

	input := huh.NewInput().
		Title(fieldTitle(field)).
		Value(&value)

	if field.Placeholder != "" {
		input.Placeholder(field.Placeholder)
	}

	if field.Required {
		input.Validate(requiredValidator(field.Label))
	}

	return input, &value
}

func createTextField(field config.TemplateField, prefilledVal any, hasPrefilled bool) (huh.Field, any) {
	var value string
	if hasPrefilled {
		if s, ok := prefilledVal.(string); ok {
			value = s
		}
	} else if field.Default != "" {
		value = field.Default
	}

	text := huh.NewText().
		Title(fieldTitle(field)).
		Value(&value)

	if field.Placeholder != "" {
		text.Placeholder(field.Placeholder)
	}

	if field.Required {
		text.Validate(requiredValidator(field.Label))
	}

	return text, &value
}

func createSelectField(field config.TemplateField, prefilledVal any, hasPrefilled bool) (huh.Field, any) {
	var value string
	if hasPrefilled {
		if s, ok := prefilledVal.(string); ok {
			value = s
		}
	} else if field.Default != "" {
		value = field.Default
	}

	options := make([]huh.Option[string], len(field.Options))
	for i, opt := range field.Options {
		label := opt.Label
		if label == "" {
			label = opt.Value
		}
		options[i] = huh.NewOption(label, opt.Value)
	}

	sel := huh.NewSelect[string]().
		Title(fieldTitle(field)).
		Options(options...).
		Value(&value)

	return sel, &value
}

func createMultiSelectField(field config.TemplateField, prefilledVal any, hasPrefilled bool) (huh.Field, any) {
	var values []string
	if hasPrefilled {
		if arr, ok := prefilledVal.([]string); ok {
			values = arr
		}
	}

	options := make([]huh.Option[string], len(field.Options))
	for i, opt := range field.Options {
		label := opt.Label
		if label == "" {
			label = opt.Value
		}
		options[i] = huh.NewOption(label, opt.Value)
	}

	multi := huh.NewMultiSelect[string]().
		Title(fieldTitle(field)).
		Options(options...).
		Value(&values)

	return multi, &values
}

// fieldTitle generates the display title for a field.
func fieldTitle(field config.TemplateField) string {
	title := field.Label
	if title == "" {
		title = field.Name
	}
	if field.Required {
		title += " *"
	}
	return title
}

// requiredValidator returns a validator that checks for non-empty values.
func requiredValidator(label string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", label)
		}
		return nil
	}
}

// extractValue extracts the actual value from a binding pointer.
func extractValue(binding any) any {
	switch v := binding.(type) {
	case *string:
		return *v
	case *[]string:
		return *v
	default:
		return nil
	}
}

// ParseSetValues parses --set flag values into a map.
// Format: "name=value" or "name=val1,val2" for multi-select.
func ParseSetValues(sets []string) (map[string]any, error) {
	result := make(map[string]any)

	for _, s := range sets {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --set format %q: expected name=value", s)
		}

		name := strings.TrimSpace(parts[0])
		value := parts[1]

		if name == "" {
			return nil, fmt.Errorf("invalid --set format %q: empty name", s)
		}

		// Check if value contains commas (multi-value)
		if strings.Contains(value, ",") {
			values := strings.Split(value, ",")
			for i := range values {
				values[i] = strings.TrimSpace(values[i])
			}
			result[name] = values
		} else {
			result[name] = value
		}
	}

	return result, nil
}

// ValidateRequiredFields checks that all required template fields have values.
func ValidateRequiredFields(tmpl config.Template, values map[string]any) error {
	for _, field := range tmpl.Fields {
		if !field.Required {
			continue
		}

		v, ok := values[field.Name]
		if !ok {
			return fmt.Errorf("required field %q is missing", field.Name)
		}

		// Check for empty values
		switch val := v.(type) {
		case string:
			if strings.TrimSpace(val) == "" {
				return fmt.Errorf("required field %q is empty", field.Name)
			}
		case []string:
			if len(val) == 0 {
				return fmt.Errorf("required field %q has no selections", field.Name)
			}
		}
	}

	return nil
}
