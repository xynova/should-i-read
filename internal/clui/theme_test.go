package clui_test

import (
	"testing"

	"github.com/xynova/should-i-read/internal/clui"
)

func TestThemeAccentFromEnv(t *testing.T) {
	t.Setenv("OPERATOR_CLI_THEME_ACCENT", "39")
	t.Setenv("OPERATOR_CLI_THEME_MARK", "220")
	if clui.AccentColorIndex() != "39" {
		t.Fatalf("accent: got %q", clui.AccentColorIndex())
	}
	if clui.MarkColorIndex() != "220" {
		t.Fatalf("mark: got %q", clui.MarkColorIndex())
	}
}
