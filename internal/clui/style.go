package clui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mitchellh/go-wordwrap"
	"golang.org/x/term"
)

const defaultBoxTerminalWidth = 80

// boxTerminalWidthFunc returns the max outer width for FormatBox (inject in tests).
var boxTerminalWidthFunc = defaultBoxTerminalWidthFunc

func defaultBoxTerminalWidthFunc() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w < 20 {
		return defaultBoxTerminalWidth
	}
	return w
}

func colorEnabled() bool {
	return strings.TrimSpace(os.Getenv("NO_COLOR")) == ""
}

func accentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(themeAccent()))
}

func markStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(themeMark()))
}

func mutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
}

func errStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
}

func missStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
}

func boxOuterWidth() int {
	return boxTerminalWidthFunc()
}

// BoxContentWidth is the target width for wrapped lines inside a FormatBox frame.
func BoxContentWidth() int {
	const frameOverhead = 6
	w := boxOuterWidth()
	inner := w - frameOverhead
	if inner < 20 {
		return defaultBoxTerminalWidth - frameOverhead
	}
	return inner
}

func boxStyle(outerWidth int) lipgloss.Style {
	s := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(themeMark())).
		Padding(0, 1).
		MarginBottom(1)
	if outerWidth > 0 {
		s = s.MaxWidth(outerWidth)
	}
	return s
}

// WrapToWidth word-wraps s to width (no-op when width <= 0).
func WrapToWidth(s string, width int) string {
	if width <= 0 || strings.TrimSpace(s) == "" {
		return s
	}
	return wordwrap.WrapString(s, uint(width))
}

func wrapBoxText(text string, width int) string {
	if width <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = wordwrap.WrapString(line, uint(width))
	}
	return strings.Join(lines, "\n")
}

// Title renders a section title.
func Title(s string) string {
	if !colorEnabled() {
		return s
	}
	return accentStyle().Render(s)
}

// OK renders success text.
func OK(s string) string {
	if !colorEnabled() {
		return s
	}
	return accentStyle().Render(s)
}

// Miss renders failure/missing text.
func Miss(s string) string {
	if !colorEnabled() {
		return s
	}
	return missStyle().Render(s)
}

// Muted renders secondary text.
func Muted(s string) string {
	if !colorEnabled() {
		return s
	}
	return mutedStyle().Render(s)
}

// Err renders error emphasis.
func Err(s string) string {
	if !colorEnabled() {
		return s
	}
	return errStyle().Render(s)
}

// Hint renders a hint line.
func Hint(s string) string {
	if !colorEnabled() {
		return s
	}
	return mutedStyle().Render(s)
}

// Label renders a field label (e.g. Subject:).
func Label(s string) string {
	if !colorEnabled() {
		return s
	}
	return accentStyle().Render(s)
}

// Mark renders a branded emphasis (task bullets, secondary highlights).
func Mark(s string) string {
	if !colorEnabled() {
		return s
	}
	return markStyle().Render(s)
}

// FormatBox wraps body under a title in a rounded box.
func FormatBox(title, body string) string {
	outer := boxOuterWidth()
	contentW := BoxContentWidth()
	var b strings.Builder
	b.WriteString(Title(title))
	b.WriteByte('\n')
	b.WriteString(strings.TrimRight(wrapBoxText(body, contentW), "\n"))
	text := b.String()
	if !colorEnabled() {
		return text
	}
	return boxStyle(outer).Render(text)
}

func longestPhysicalLine(s string) int {
	max := 0
	for _, line := range strings.Split(s, "\n") {
		w := lipgloss.Width(line)
		if w > max {
			max = w
		}
	}
	return max
}

// MarkOK returns a check mark string.
func MarkOK() string {
	return OK("✓")
}

// MarkMiss returns a cross mark string.
func MarkMiss() string {
	return Miss("✗")
}
