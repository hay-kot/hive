package terminal

import (
	"strings"
)

// Detector detects AI tool status from terminal content.
type Detector struct {
	tool string
}

// NewDetector creates a detector for the specified tool.
func NewDetector(tool string) *Detector {
	return &Detector{tool: strings.ToLower(tool)}
}

// spinnerChars are braille and asterisk spinner characters used by Claude Code.
// Includes both the classic braille dots and the Claude 2.1.25+ asterisk chars.
var spinnerChars = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", // braille dots
	"✳", "✽", "✶", "✢", // Claude 2.1.25+ asterisk spinner
}

// whimsicalWords are the "thinking" words Claude Code displays during processing.
// These appear with spinners like "⠋ pondering..." or "✳ clauding..."
var whimsicalWords = []string{
	"accomplishing", "actioning", "actualizing", "baking", "booping",
	"brewing", "calculating", "cerebrating", "channelling", "churning",
	"clauding", "coalescing", "cogitating", "combobulating", "computing",
	"concocting", "conjuring", "considering", "contemplating", "cooking",
	"crafting", "creating", "crunching", "deciphering", "deliberating",
	"determining", "discombobulating", "divining", "doing", "effecting",
	"elucidating", "enchanting", "envisioning", "finagling", "flibbertigibbeting",
	"forging", "forming", "frolicking", "generating", "germinating",
	"hatching", "herding", "honking", "hustling", "ideating",
	"imagining", "incubating", "inferring", "jiving", "manifesting",
	"marinating", "meandering", "moseying", "mulling", "mustering",
	"musing", "noodling", "percolating", "perusing", "philosophising",
	"pondering", "pontificating", "processing", "puttering", "puzzling",
	"reticulating", "ruminating", "scheming", "schlepping", "shimmying",
	"shucking", "simmering", "smooshing", "spelunking", "spinning",
	"stewing", "sussing", "synthesizing", "thinking", "tinkering",
	"transmuting", "unfurling", "unravelling", "vibing", "wandering",
	"whirring", "wibbling", "wizarding", "working", "wrangling",
	"billowing", "gusting", "metamorphosing", "sublimating", "recombobulating", "sautéing",
}

// IsBusy returns true if the terminal content indicates the agent is actively working.
func (d *Detector) IsBusy(content string) bool {
	// Check last 15 lines for context (matches Agent Deck)
	lines := getLastNonEmptyLines(content, 15)
	recentContent := strings.Join(lines, "\n")
	recentLower := strings.ToLower(recentContent)

	// Check for explicit busy indicators (most reliable)
	busyIndicators := []string{
		"ctrl+c to interrupt",
		"esc to interrupt",
	}
	for _, indicator := range busyIndicators {
		if strings.Contains(recentLower, indicator) {
			return true
		}
	}

	// Check for spinner characters in recent lines
	for _, line := range lines {
		// Skip lines starting with box-drawing characters (UI borders)
		trimmedLine := strings.TrimSpace(line)
		if len(trimmedLine) > 0 {
			r := []rune(trimmedLine)[0]
			if isBoxDrawingChar(r) {
				continue
			}
		}
		for _, spinner := range spinnerChars {
			if strings.Contains(line, spinner) {
				return true
			}
		}
	}

	// Check for whimsical thinking words with ellipsis in recent content only
	// These must appear at the start of a line or after a spinner (status line format)
	for _, line := range lines {
		lineLower := strings.ToLower(strings.TrimSpace(line))
		for _, word := range whimsicalWords {
			// Check for word followed by ellipsis at reasonable position
			pattern := word + "…"
			patternAscii := word + "..."
			if strings.HasPrefix(lineLower, pattern) || strings.HasPrefix(lineLower, patternAscii) {
				return true
			}
			// Also check after spinner chars (e.g., "⠙ pondering…")
			for _, spinner := range spinnerChars {
				if strings.Contains(line, spinner+" "+word) {
					return true
				}
			}
		}
	}

	// Check for thinking indicator with timing info (e.g., "Thinking... (45s · 1234 tokens)")
	if strings.Contains(recentLower, "thinking") && strings.Contains(recentLower, "tokens") {
		return true
	}
	if strings.Contains(recentLower, "connecting") && strings.Contains(recentLower, "tokens") {
		return true
	}

	return false
}

// isBoxDrawingChar returns true if the rune is a box-drawing character.
func isBoxDrawingChar(r rune) bool {
	return r == '│' || r == '├' || r == '└' || r == '─' || r == '┌' ||
		r == '┐' || r == '┘' || r == '┤' || r == '┬' || r == '┴' ||
		r == '┼' || r == '╭' || r == '╰' || r == '╮' || r == '╯'
}

// IsWaiting returns true if the terminal content indicates the agent needs user input.
func (d *Detector) IsWaiting(content string) bool {
	// If busy, not waiting
	if d.IsBusy(content) {
		return false
	}

	lines := getLastNonEmptyLines(content, 15)
	recentContent := strings.Join(lines, "\n")

	// Permission prompts (normal mode)
	permissionPrompts := []string{
		// Primary Claude Squad indicator
		"No, and tell Claude what to do differently",
		// Permission dialog options
		"Yes, allow once",
		"Yes, allow always",
		"Allow once",
		"Allow always",
		// Box-drawing permission dialogs
		"│ Do you want",
		"│ Would you like",
		"│ Allow",
		// Selection indicators
		"❯ Yes",
		"❯ No",
		"❯ Allow",
		// Trust prompt on startup
		"Do you trust the files in this folder?",
		// MCP permission prompts
		"Allow this MCP server",
		// Tool permission prompts
		"Run this command?",
		"Execute this?",
		"Action Required",
		"Waiting for user confirmation",
		"Allow execution of",
		// AskUserQuestion / interactive question UI
		"Use arrow keys to navigate",
		"Press Enter to select",
	}
	for _, prompt := range permissionPrompts {
		if strings.Contains(recentContent, prompt) {
			return true
		}
	}

	// Check for standalone prompt character in last few lines
	// Claude Code's UI has status bar AFTER the prompt, so check multiple lines
	checkLines := lines
	if len(checkLines) > 5 {
		checkLines = checkLines[len(checkLines)-5:]
	}
	for _, line := range checkLines {
		cleanLine := strings.TrimSpace(stripANSI(line))
		// Normalize non-breaking spaces (U+00A0) to regular spaces
		cleanLine = strings.ReplaceAll(cleanLine, "\u00A0", " ")

		// Claude Code shows ">" or "❯" when waiting for input
		if cleanLine == ">" || cleanLine == "❯" || cleanLine == "> " || cleanLine == "❯ " {
			return true
		}

		// Check for prompt with suggestion (Claude shows "❯ Try..." when waiting)
		if strings.HasPrefix(cleanLine, "❯ Try ") || strings.HasPrefix(cleanLine, "> Try ") {
			return true
		}
	}

	// Yes/No confirmation prompts
	confirmPatterns := []string{
		"(Y/n)", "[Y/n]", "(y/N)", "[y/N]",
		"(yes/no)", "[yes/no]",
		"Continue?", "Proceed?",
		"Approve this plan?",
		"Execute plan?",
	}
	for _, pattern := range confirmPatterns {
		if strings.Contains(recentContent, pattern) {
			return true
		}
	}

	// Check for completion indicators combined with prompt
	// When Claude finishes, it shows summary and waits for next input
	completionIndicators := []string{
		"task completed",
		"done!",
		"finished",
		"what would you like",
		"what else",
		"anything else",
		"let me know if",
	}
	recentLower := strings.ToLower(recentContent)
	hasCompletion := false
	for _, indicator := range completionIndicators {
		if strings.Contains(recentLower, indicator) {
			hasCompletion = true
			break
		}
	}
	if hasCompletion {
		// Check if there's a prompt nearby
		last3 := lines
		if len(last3) > 3 {
			last3 = last3[len(last3)-3:]
		}
		for _, line := range last3 {
			cleanLine := strings.TrimSpace(stripANSI(line))
			cleanLine = strings.ReplaceAll(cleanLine, "\u00A0", " ")
			if cleanLine == ">" || cleanLine == "❯" || cleanLine == "> " || cleanLine == "❯ " {
				return true
			}
		}
	}

	return false
}

// DetectStatus returns the detected status based on terminal content alone.
// For more accurate detection with spike filtering, use StateTracker.Update().
func (d *Detector) DetectStatus(content string) Status {
	if d.IsBusy(content) {
		return StatusActive
	}
	if d.IsWaiting(content) {
		return StatusWaiting
	}
	return StatusIdle
}

// DetectTool attempts to identify the AI tool from terminal content.
func DetectTool(content string) string {
	lower := strings.ToLower(content)

	patterns := map[string][]string{
		"claude": {
			"claude",
			"anthropic",
			"ctrl+c to interrupt",
		},
		"gemini": {
			"gemini",
			"google ai",
		},
		"opencode": {
			"opencode",
			"open code",
		},
		"codex": {
			"codex",
			"openai",
		},
	}

	for tool, keywords := range patterns {
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				return tool
			}
		}
	}

	return "shell"
}

// getLastNonEmptyLines returns the last n non-empty lines from content.
func getLastNonEmptyLines(content string, n int) []string {
	lines := strings.Split(content, "\n")
	var result []string

	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			result = append([]string{lines[i]}, result...)
		}
	}

	return result
}

// stripANSI removes ANSI escape codes from content.
func stripANSI(content string) string {
	// Fast path: if no escape chars, return as-is
	if !strings.Contains(content, "\x1b") && !strings.Contains(content, "\x9B") {
		return content
	}

	var b strings.Builder
	b.Grow(len(content))

	i := 0
	for i < len(content) {
		if content[i] == '\x1b' {
			// CSI sequence: ESC [ ... letter
			if i+1 < len(content) && content[i+1] == '[' {
				j := i + 2
				for j < len(content) {
					c := content[j]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
						j++
						break
					}
					j++
				}
				i = j
				continue
			}
			// OSC sequence: ESC ] ... BEL or ST
			if i+1 < len(content) && content[i+1] == ']' {
				bellPos := strings.Index(content[i:], "\x07")
				if bellPos != -1 {
					i += bellPos + 1
					continue
				}
				// Check for ST (ESC \) as alternative terminator
				stPos := strings.Index(content[i:], "\x1b\\")
				if stPos != -1 {
					i += stPos + 2
					continue
				}
			}
			// Other escape: skip 2 chars
			if i+1 < len(content) {
				i += 2
				continue
			}
		}
		if content[i] == '\x9B' {
			j := i + 1
			for j < len(content) {
				c := content[j]
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		b.WriteByte(content[i])
		i++
	}

	return b.String()
}
