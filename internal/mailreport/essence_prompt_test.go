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

func TestParseEssenceResponse_kindSlug(t *testing.T) {
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: x\nSHAPE: y\nKIND: legal-notification > case-update > Legal notice\n"
	out, err := parseEssenceResponse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "legal-notification > case-update > legal-notice"
	if out.Kind != want {
		t.Fatalf("kind: got %q want %q", out.Kind, want)
	}
}

func TestEssenceIntegrityCompleteNote_namedWhyKeys(t *testing.T) {
	raw := "WHY5: By highlighting the value of the recipient's ideas.\nABOUT: professional development\nSHAPE: rectangular\nKIND: communication > professional-networking > business\n"
	_, err := parseEssenceResponse(raw)
	if err == nil {
		t.Fatal("expected parse error")
	}
	note := essenceIntegrityCompleteNote(err)
	if !strings.Contains(note, "Missing: WHY1, WHY2, WHY3, WHY4.") {
		t.Fatalf("missing keys: %q", note)
	}
	if !strings.Contains(note, "Keep WHY5, ABOUT, SHAPE, KIND.") {
		t.Fatalf("keep keys: %q", note)
	}
	if strings.Contains(strings.ToLower(note), "start over") {
		t.Fatalf("note must not discard prior: %q", note)
	}
}

func TestEssenceIntegrityCompleteNote_kindSegments(t *testing.T) {
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: x\nSHAPE: y\nKIND: gitlab\n"
	_, err := parseEssenceResponse(raw)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !essenceKindShortFromErr(err) {
		t.Fatal("expected kind_short field")
	}
	note := essenceIntegrityCompleteNote(err)
	if !strings.Contains(strings.ToLower(note), "continue") {
		t.Fatalf("note %q", note)
	}
	if !strings.Contains(note, "Expand KIND") {
		t.Fatalf("note %q", note)
	}
}
