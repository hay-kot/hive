// Package tmpl provides template rendering utilities for shell commands.
package tmpl

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// shellQuote returns a shell-safe quoted string. It wraps the string in single
// quotes and escapes any existing single quotes using the '\” technique.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Replace ' with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(s, "'", `'\''`)
	return "'" + escaped + "'"
}

var funcs = template.FuncMap{
	"shq": shellQuote,
}

// Render executes a Go template string with the given data.
// Returns an error if the template is invalid or references undefined keys.
//
// Available template functions:
//   - shq: Shell-quote a string for safe use in shell commands
func Render(tmpl string, data any) (string, error) {
	t, err := template.New("").Funcs(funcs).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}
