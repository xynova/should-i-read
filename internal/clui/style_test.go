package clui

import (
	"strings"
	"testing"
)

func TestFormatBoxCapsWidth(t *testing.T) {
	const outer = 80
	prev := boxTerminalWidthFunc
	boxTerminalWidthFunc = func() int { return outer }
	defer func() { boxTerminalWidthFunc = prev }()

	t.Setenv("NO_COLOR", "")

	long := strings.Repeat("word ", 60)
	out := FormatBox("Classify", long)
	if longestPhysicalLine(out) > outer {
		t.Fatalf("line wider than %d: %d\n%s", outer, longestPhysicalLine(out), out)
	}
}

func TestBoxContentWidthFallback(t *testing.T) {
	prev := boxTerminalWidthFunc
	boxTerminalWidthFunc = func() int { return 10 }
	defer func() { boxTerminalWidthFunc = prev }()
	if got, want := BoxContentWidth(), defaultBoxTerminalWidth-6; got != want {
		t.Fatalf("content width: got %d want %d", got, want)
	}
}
