package mailreport

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/mailtags"
)

func testTagCatalog(t *testing.T) *mailtags.Catalog {
	seed := filepath.Join("..", "..", "config", "vocabularies", "mail-tags-seed.txt")
	cat, err := mailtags.CreateCatalog(mailtags.Config{SeedPath: seed})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return cat
}

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
	cat := testTagCatalog(t)
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: x\nTAGS: marketing-pitch promotional\nKIND: legal-notification > case-update > Legal notice\n"
	out, err := parseEssenceResponseWithCatalog(raw, cat)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "legal-notification > case-update > legal-notice"
	if out.Kind != want {
		t.Fatalf("kind: got %q want %q", out.Kind, want)
	}
	if out.Shape != "marketing-pitch promotional" {
		t.Fatalf("shape: %q", out.Shape)
	}
}

func TestParseEssenceResponse_rejectsRectangular(t *testing.T) {
	cat := testTagCatalog(t)
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: x\nSHAPE: rectangular\nKIND: communication > professional-networking > business\n"
	_, err := parseEssenceResponseWithCatalog(raw, cat)
	if err == nil {
		t.Fatal("expected tag invalid")
	}
}

func TestParseEssenceResponse_mintKebab(t *testing.T) {
	cat := testTagCatalog(t)
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: x\nTAGS: solicitation\nKIND: communication > professional-networking > business\n"
	out, err := parseEssenceResponseWithCatalog(raw, cat)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Shape != "solicitation" {
		t.Fatalf("shape %q", out.Shape)
	}
}

func TestEssenceIntegrityCompleteNote_namedWhyKeys(t *testing.T) {
	cat := testTagCatalog(t)
	raw := "WHY5: By highlighting the value of the recipient's ideas.\nABOUT: professional development\nTAGS: fyi\nKIND: communication > professional-networking > business\n"
	_, err := parseEssenceResponseWithCatalog(raw, cat)
	if err == nil {
		t.Fatal("expected parse error")
	}
	note := essenceIntegrityCompleteNote(err)
	if !strings.Contains(note, "Missing: WHY1, WHY2, WHY3, WHY4.") {
		t.Fatalf("missing keys: %q", note)
	}
	if !strings.Contains(note, "Keep WHY5, ABOUT, TAGS, KIND.") {
		t.Fatalf("keep keys: %q", note)
	}
	if strings.Contains(strings.ToLower(note), "start over") {
		t.Fatalf("note must not discard prior: %q", note)
	}
}

func TestEssenceIntegrityCompleteNote_kindSegments(t *testing.T) {
	cat := testTagCatalog(t)
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: x\nTAGS: fyi\nKIND: gitlab\n"
	_, err := parseEssenceResponseWithCatalog(raw, cat)
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
