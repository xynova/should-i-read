package clui

import (
	"os"
	"strings"
)

// OPERATOR_CLI_THEME_ACCENT and OPERATOR_CLI_THEME_MARK are xterm-256 color indices
// (0–255) for operator CLI branding. Host Taskfiles SHOULD map the same values to
// TASK_COLOR_GREEN and TASK_COLOR_YELLOW (38;5;<n>) so go tool task --list matches
// the binary UI. Other operator CLIs MAY read the same env names.
const (
	envThemeAccent = "OPERATOR_CLI_THEME_ACCENT"
	envThemeMark   = "OPERATOR_CLI_THEME_MARK"

	defaultThemeAccent = "42"
	defaultThemeMark   = "214"
)

func themeAccent() string {
	if v := strings.TrimSpace(os.Getenv(envThemeAccent)); v != "" {
		return v
	}
	return defaultThemeAccent
}

func themeMark() string {
	if v := strings.TrimSpace(os.Getenv(envThemeMark)); v != "" {
		return v
	}
	return defaultThemeMark
}

// AccentColorIndex returns the active xterm-256 accent index (for tests and tooling).
func AccentColorIndex() string { return themeAccent() }

// MarkColorIndex returns the active xterm-256 mark index (for tests and tooling).
func MarkColorIndex() string { return themeMark() }
