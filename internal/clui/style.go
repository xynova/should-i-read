package clui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	styleOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	styleMiss  = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	styleMuted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleErr   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	styleHint  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleBox   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1).
			MarginBottom(1)
)

func colorEnabled() bool {
	return strings.TrimSpace(os.Getenv("NO_COLOR")) == ""
}

// Title renders a section title.
func Title(s string) string {
	if !colorEnabled() {
		return s
	}
	return styleTitle.Render(s)
}

// OK renders success text.
func OK(s string) string {
	if !colorEnabled() {
		return s
	}
	return styleOK.Render(s)
}

// Miss renders failure/missing text.
func Miss(s string) string {
	if !colorEnabled() {
		return s
	}
	return styleMiss.Render(s)
}

// Muted renders secondary text.
func Muted(s string) string {
	if !colorEnabled() {
		return s
	}
	return styleMuted.Render(s)
}

// Err renders error emphasis.
func Err(s string) string {
	if !colorEnabled() {
		return s
	}
	return styleErr.Render(s)
}

// Hint renders a hint line.
func Hint(s string) string {
	if !colorEnabled() {
		return s
	}
	return styleHint.Render(s)
}

// Label renders a field label (e.g. Subject:).
func Label(s string) string {
	if !colorEnabled() {
		return s
	}
	return styleLabel.Render(s)
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
	return styleBox.Render(text)
}

// MarkOK returns a check mark string.
func MarkOK() string {
	return OK("✓")
}

// MarkMiss returns a cross mark string.
func MarkMiss() string {
	return Miss("✗")
}
