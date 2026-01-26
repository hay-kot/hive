package printer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hay-kot/criterio"
)

// ANSI color codes (Tokyo Night palette)
const (
	ColorReset     = "\033[0m"
	ColorRed       = "\033[38;2;215;95;107m"  // #d75f6b
	ColorGreen     = "\033[38;2;158;206;106m" // #9ece6a (Tokyo Night green)
	ColorYellow    = "\033[38;2;224;175;104m" // #e0af68 (Tokyo Night yellow)
	ColorGray      = "\033[38;2;86;95;137m"   // #565f89 (Tokyo Night comment)
	ColorBold      = "\033[1m"
	ColorUnderline = "\033[4m"
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

	// Check if the error contains criterio.FieldErrors for better formatting
	var fieldErrs criterio.FieldErrors
	if errors.As(err, &fieldErrs) {
		p.printValidationErrors(err, fieldErrs)
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

// printValidationErrors formats criterio.FieldErrors nicely
func (p *Printer) printValidationErrors(wrappedErr error, fieldErrs criterio.FieldErrors) {
	// Extract the context from the wrapped error (e.g., "load config: invalid config:")
	errStr := wrappedErr.Error()
	fieldErrStr := fieldErrs.Error()

	// Find where the field errors start in the wrapped error
	errContext := ""
	if idx := strings.Index(errStr, fieldErrStr); idx > 0 {
		errContext = strings.TrimSuffix(errStr[:idx], ": ")
	}

	_, _ = p.writer.Write([]byte(p.colorize(ColorRed, "╭ Validation Error") + "\n"))

	if errContext != "" {
		_, _ = p.writer.Write([]byte(p.colorize(ColorRed, "│") + " " + p.colorize(ColorGray, errContext) + "\n"))
		_, _ = p.writer.Write([]byte(p.colorize(ColorRed, "│") + "\n"))
	}

	for _, fe := range fieldErrs {
		line := p.colorize(ColorRed, "│") + " " + p.colorize(ColorRed, Cross) + " "
		if fe.Field != "" {
			line += p.colorize(ColorGray, fe.Field+": ")
		}
		line += fe.Err.Error()
		_, _ = p.writer.Write([]byte(line + "\n"))
	}

	_, _ = p.writer.Write([]byte(p.colorize(ColorRed, "╵") + "\n"))
}

// Errorf prints an error message in red
func (p *Printer) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = p.writer.Write([]byte(p.colorize(ColorRed, Cross+" "+msg) + "\n"))
}

// Successf prints a success message in green
func (p *Printer) Successf(format string, args ...any) {
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
func (p *Printer) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = p.writer.Write([]byte(p.colorize(ColorGray, Dot+" "+msg) + "\n"))
}

// Warnf prints a warning message in yellow
func (p *Printer) Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = p.writer.Write([]byte(p.colorize(ColorYellow, Dot+" "+msg) + "\n"))
}

// Printf prints a plain message without colors
func (p *Printer) Printf(format string, args ...any) {
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

// Section prints a section header (bold + underlined)
func (p *Printer) Section(title string) {
	_, _ = p.writer.Write([]byte(ColorBold + ColorUnderline + title + ColorReset + "\n"))
}

// CheckItem prints a success item with green checkmark
func (p *Printer) CheckItem(label, detail string) {
	p.printItem(ColorGreen, Check, label, detail)
}

// WarnItem prints a warning item with yellow dot
func (p *Printer) WarnItem(label, detail string) {
	p.printItem(ColorYellow, Dot, label, detail)
}

// FailItem prints a failure item with red cross
func (p *Printer) FailItem(label, detail string) {
	p.printItem(ColorRed, Cross, label, detail)
}

func (p *Printer) printItem(color, symbol, label, detail string) {
	line := "  " + p.colorize(color, symbol) + " " + label
	if detail != "" {
		line += ": " + detail
	}
	_, _ = p.writer.Write([]byte(line + "\n"))
}

// StatusOK returns a green checkmark with "ok" for use in tables.
func StatusOK() string {
	return ColorGreen + Check + ColorReset + " ok"
}

// StatusFailed returns a red cross with the given message for use in tables.
func StatusFailed(msg string) string {
	return ColorRed + Cross + ColorReset + " " + msg
}

// StatusWarn returns a yellow dot with the given message for use in tables.
func StatusWarn(msg string) string {
	return ColorYellow + Dot + ColorReset + " " + msg
}
