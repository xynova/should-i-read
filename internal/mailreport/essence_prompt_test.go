package mailreport

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/strop/pkg/integrity"
	"github.com/behaviorengineering/taxonomy/pkg/tags"
)

func testTagCatalog(t *testing.T) *tags.Catalog {
	seed := filepath.Join("..", "..", "config", "vocabularies", "mail-tags-seed.txt")
	cat, err := tags.CreateCatalog(MailTagCatalogConfig("", "", seed))
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

func TestParseEssenceResponse_kindPathTagsDropsDeny(t *testing.T) {
	cat := testTagCatalog(t)
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: Google security alert\nTAGS: security > alert > google > notification > email\nKIND: security > alert > notification > email\n"
	out, err := parseEssenceResponseWithCatalog(raw, cat)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.Contains(out.Shape, "email") {
		t.Fatalf("shape kept deny token: %q", out.Shape)
	}
	if !strings.Contains(out.Shape, "security") || !strings.Contains(out.Shape, "notification") {
		t.Fatalf("shape dropped valid tokens: %q", out.Shape)
	}
}

func TestParseEssenceResponse_kindPathTags(t *testing.T) {
	cat := testTagCatalog(t)
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: Auth0 free trial\nTAGS: auth0 > trial > notification > service-alert > transactional\nKIND: notification > service-alert > transactional\n"
	out, err := parseEssenceResponseWithCatalog(raw, cat)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "auth0 notification service-alert transactional trial"
	if out.Shape != want {
		t.Fatalf("shape: got %q want %q", out.Shape, want)
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
	if !strings.Contains(note, "Add WHY1, WHY2, and WHY3") {
		t.Fatalf("add first whys: %q", note)
	}
	if !strings.Contains(note, "do not rename WHY4 to WHY1") {
		t.Fatalf("keep later why keys: %q", note)
	}
	if !strings.Contains(note, "WHY1, WHY2, WHY3, WHY4") {
		t.Fatalf("missing keys: %q", note)
	}
	if strings.Contains(strings.ToLower(note), "start over") {
		t.Fatalf("note must not discard prior: %q", note)
	}
}

func TestEssenceIntegrityAfterMiss_why1EmptyRestarts(t *testing.T) {
	cat := testTagCatalog(t)
	why4First := "WHY4: delivered promptly\nWHY5: short layout\nTAGS: job-alert\nKIND: notification > job-alert > software-engineering\n"
	_, err := parseEssenceResponseWithCatalog(why4First, cat)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if essenceIntegrityAfterMiss(err) != integrity.MissRestart {
		t.Fatalf("expected MissRestart for WHY1 empty, got %v", essenceIntegrityAfterMiss(err))
	}
	whyOnly := "WHY1: w\nWHY2: w\nWHY3: w\nWHY4: w\nWHY5: w\n"
	_, err = parseEssenceResponseWithCatalog(whyOnly, cat)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if essenceIntegrityAfterMiss(err) != integrity.MissContinue {
		t.Fatalf("expected MissContinue when WHY1 present, got %v", essenceIntegrityAfterMiss(err))
	}
}

func TestEssenceIntegrityCompleteNote_startedAtWhy4(t *testing.T) {
	cat := testTagCatalog(t)
	raw := "WHY4: delivered promptly\nWHY5: short layout\nTAGS: job-alert\nKIND: notification > job-alert > software-engineering\n"
	_, err := parseEssenceResponseWithCatalog(raw, cat)
	if err == nil {
		t.Fatal("expected parse error")
	}
	note := essenceIntegrityCompleteNote(err)
	if !strings.Contains(note, "do not rename WHY4 to WHY1") {
		t.Fatalf("note %q", note)
	}
	if !strings.Contains(note, "WHY1, WHY2, WHY3") {
		t.Fatalf("note %q", note)
	}
}

func TestParseEssenceResponse_kindTrailingArrow(t *testing.T) {
	cat := testTagCatalog(t)
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: CI run failed\nTAGS: ci failure\nKIND: ci >\n"
	_, err := parseEssenceResponseWithCatalog(raw, cat)
	if err == nil {
		t.Fatal("expected kind short")
	}
	if !essenceKindShortFromErr(err) {
		t.Fatalf("err %v", err)
	}
	note := essenceIntegrityCompleteNote(err)
	if !strings.Contains(note, `trailing " > "`) {
		t.Fatalf("note %q", note)
	}
}

func TestDefaultEssenceSystemPrompt_whyOrderAndKind(t *testing.T) {
	t.Parallel()
	p := DefaultEssenceSystemPrompt()
	if !strings.Contains(p, "Never start at WHY4") {
		t.Fatalf("prompt missing WHY order: %s", p)
	}
	if !strings.Contains(p, `trailing " > "`) {
		t.Fatalf("prompt missing KIND trailing rule: %s", p)
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
	if !strings.Contains(note, `trailing " > "`) {
		t.Fatalf("note %q", note)
	}
}
