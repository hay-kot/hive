package printer

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// ANSI color codes (Tokyo Night palette)
const (
	ColorReset = "\033[0m"
	ColorRed   = "\033[38;2;215;95;107m"  // #d75f6b
	ColorGreen = "\033[38;2;158;206;106m" // #9ece6a (Tokyo Night green)
	ColorGray  = "\033[38;2;86;95;137m"   // #565f89 (Tokyo Night comment)
	ColorBold  = "\033[1m"
)

// Symbols
const (
	Check  = "✔"
	Cross  = "✘"
	Folder = ""
	Dot    = "•"
)

type ctxKey struct{}

// Printer handles formatted output with colors and styles
type Printer struct {
	writer io.Writer
}

// New creates a new Printer that writes to the given writer
func New(w io.Writer) *Printer {
	return &Printer{
		writer: w,
	}
}

// NewContext returns a context with the printer attached
func NewContext(ctx context.Context, p *Printer) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// Ctx retrieves the printer from context, or creates a default one
func Ctx(ctx context.Context) *Printer {
	if p, ok := ctx.Value(ctxKey{}).(*Printer); ok {
		return p
	}
	return New(os.Stderr)
}

// FatalError prints a formatted error box and does NOT exit
// Caller should handle exit code
func (p *Printer) FatalError(err error) {
	if err == nil {
		return
	}

	lines := []string{
		p.colorize(ColorRed, "╭ Error"),
		p.colorize(ColorRed, "│") + " " + p.colorize(ColorGray, err.Error()),
		p.colorize(ColorRed, "╵"),
	}

	output := strings.Join(lines, "\n") + "\n"
	_, _ = p.writer.Write([]byte(output))
}

// Errorf prints an error message in red
func (p *Printer) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	_, _ = p.writer.Write([]byte(p.colorize(ColorRed, Cross+" "+msg) + "\n"))
}

// Successf prints a success message in green
func (p *Printer) Successf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	_, _ = p.writer.Write([]byte(p.colorize(ColorGreen, Check+" "+msg) + "\n"))
}

// Success prints a success message with details on a separate line
func (p *Printer) Success(message string, details string) {
	_, _ = p.writer.Write([]byte(p.colorize(ColorGreen, Check+" "+message) + "\n"))
	if details != "" {
		_, _ = p.writer.Write([]byte("  " + p.colorize(ColorGray, details) + "\n"))
	}
}

// Infof prints an info message in gray
func (p *Printer) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	_, _ = p.writer.Write([]byte(p.colorize(ColorGray, Dot+" "+msg) + "\n"))
}

// Printf prints a plain message without colors
func (p *Printer) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	_, _ = p.writer.Write([]byte(msg + "\n"))
}

// colorize applies ANSI color codes to text
func (p *Printer) colorize(color, text string) string {
	return color + text + ColorReset
}

// Bold makes text bold
func (p *Printer) Bold(text string) string {
	return ColorBold + text + ColorReset
}
