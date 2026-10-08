package mailreport

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEssenceBody_capsRunes(t *testing.T) {
	long := strings.Repeat("word ", 300)
	got := EssenceBody(long)
	if utf8.RuneCountInString(got) > essenceMaxRunes {
		t.Fatalf("want at most %d runes, got %d", essenceMaxRunes, utf8.RuneCountInString(got))
	}
}

func TestEssenceBody_firstParagraphs(t *testing.T) {
	in := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	got := EssenceBody(in)
	if strings.Contains(got, "Third paragraph") {
		t.Fatalf("want only first two paragraphs: %q", got)
	}
	if !strings.Contains(got, "First paragraph") || !strings.Contains(got, "Second paragraph") {
		t.Fatalf("want first two paragraphs: %q", got)
	}
}
