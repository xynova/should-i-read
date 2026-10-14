package mailreport

import (
	"strings"
	"testing"
)

func TestEssenceIntegrityCompleteNote_aboutOrShape(t *testing.T) {
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\n"
	_, err := parseEssenceResponse(raw)
	if err == nil {
		t.Fatal("expected parse error")
	}
	note := essenceIntegrityCompleteNote(err)
	if !strings.Contains(strings.ToLower(note), "continue") {
		t.Fatalf("note %q", note)
	}
	if strings.Contains(strings.ToLower(note), "start over") || strings.Contains(strings.ToLower(note), "rewrite") {
		t.Fatalf("note must not discard prior: %q", note)
	}
	if !strings.Contains(note, "ABOUT") {
		t.Fatalf("note %q", note)
	}
}

func TestEssenceIntegrityCompleteNote_kindSegments(t *testing.T) {
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: x\nSHAPE: y\nKIND: gitlab\n"
	_, err := parseEssenceResponse(raw)
	if err == nil {
		t.Fatal("expected parse error")
	}
	note := essenceIntegrityCompleteNote(err)
	if !strings.Contains(strings.ToLower(note), "continue") {
		t.Fatalf("note %q", note)
	}
	if !strings.Contains(note, "KIND") {
		t.Fatalf("note %q", note)
	}
}
