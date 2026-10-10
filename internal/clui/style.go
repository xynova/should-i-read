package clui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

func boxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(themeMark())).
		Padding(0, 1).
		MarginBottom(1)
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
	var b strings.Builder
	b.WriteString(Title(title))
	b.WriteByte('\n')
	b.WriteString(strings.TrimRight(body, "\n"))
	text := b.String()
	if !colorEnabled() {
		return text
	}
	return boxStyle().Render(text)
}

// MarkOK returns a check mark string.
func MarkOK() string {
	return OK("✓")
}

// MarkMiss returns a cross mark string.
func MarkMiss() string {
	return Miss("✗")
}
