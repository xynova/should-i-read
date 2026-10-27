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
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: Google security alert\nTAGS: security > alert > google > notification > email\nKIND: security > alert > notification\n"
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
	if !strings.Contains(p, "KIND: family > class > leaf") {
		t.Fatalf("prompt missing KIND template: %s", p)
	}
	if !strings.Contains(p, "exactly 3 kebab-case levels") {
		t.Fatalf("prompt missing 3-level contract: %s", p)
	}
	if strings.Contains(p, "3 to 5") {
		t.Fatalf("prompt still allows 3 to 5: %s", p)
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
	note := essenceKindIntegrityCompleteNote(err)
	if !strings.Contains(strings.ToLower(note), "continue") {
		t.Fatalf("note %q", note)
	}
	if !strings.Contains(note, "Expand KIND") {
		t.Fatalf("note %q", note)
	}
	if !strings.Contains(note, `trailing " > "`) {
		t.Fatalf("note %q", note)
	}
	if strings.Contains(note, "software-release") || strings.Contains(note, "changelog") {
		t.Fatalf("note must not leak example path: %q", note)
	}
}

func TestParseEssenceWhy_acceptsMarkdownKeys(t *testing.T) {
	t.Parallel()
	raw := "**WHY1:** New notes have been posted.\n**WHY2:** Substack posted them.\n**WHY3:** Substack is a writing platform.\n**WHY4:** Rob Stephenson uses Substack.\n**WHY5:** He records neighborhoods in New York City.\n**ABOUT:** A Substack notification that new notes were posted."
	out, err := parseEssenceWhy(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Why[0] != "New notes have been posted." {
		t.Fatalf("why1: %q", out.Why[0])
	}
	if !strings.Contains(out.About, "Substack") {
		t.Fatalf("about: %q", out.About)
	}
}

func TestParseEssenceKind_rejectsFourLevels(t *testing.T) {
	t.Parallel()
	_, err := parseEssenceKind("KIND: notification > ci > pull-request > preview-removal\n")
	if err == nil {
		t.Fatal("expected kind long")
	}
	if !essenceKindLongFromErr(err) {
		t.Fatalf("got %v", err)
	}
	note := essenceKindIntegrityCompleteNote(err)
	if !strings.Contains(note, "4th level") {
		t.Fatalf("note %q", note)
	}
	if strings.Contains(note, "software-release") || strings.Contains(note, "changelog") {
		t.Fatalf("note must not leak example path: %q", note)
	}
}

func TestParseEssenceKind_rejectsParentPrefix(t *testing.T) {
	t.Parallel()
	_, err := parseEssenceKind("KIND: notification > notification-ci > workflow-failure\n")
	if err == nil {
		t.Fatal("expected ancestor error")
	}
	if !essenceKindAncestorFromErr(err) {
		t.Fatalf("got %v", err)
	}
}

func TestParseEssenceWhy_ignoresForeignKeys(t *testing.T) {
	t.Parallel()
	raw := "WHY1: a\nTAGS: billing\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: about\nKIND: notification > billing > payment-failure\n"
	out, err := parseEssenceWhy(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Why[0] != "a" {
		t.Fatalf("why1 polluted: %q", out.Why[0])
	}
	if out.Why[1] != "b" || out.About != "about" {
		t.Fatalf("out %+v", out)
	}
}

func TestParseEssenceTags_emptyMessage(t *testing.T) {
	t.Parallel()
	_, err := parseEssenceTags("WHY1: a\n", nil)
	if err == nil {
		t.Fatal("expected tags empty")
	}
	if !strings.Contains(err.Error(), EssenceErrTagsEmpty) {
		t.Fatalf("err %v", err)
	}
	missing, _ := essenceKeysFromErr(err)
	if len(missing) != 1 || missing[0] != essenceKeyTags {
		t.Fatalf("missing %v", missing)
	}
}

func TestBuildEssencePrompts_threeSlaves(t *testing.T) {
	t.Parallel()
	p := BuildEssencePrompts([]string{"fyi", "billing"})
	if !strings.Contains(p.Why, "WHY1:") || strings.Contains(p.Why, "TAGS:") {
		t.Fatalf("why prompt: %s", p.Why)
	}
	for _, want := range []string{"event", "concrete fact", "Preserve names", "do not infer"} {
		if !strings.Contains(p.Why, want) {
			t.Fatalf("why prompt missing %q: %s", want, p.Why)
		}
	}
	if !strings.Contains(p.Tags, "TAGS:") || !strings.Contains(p.Tags, "fyi") {
		t.Fatalf("tags prompt: %s", p.Tags)
	}
	if !strings.Contains(p.Kind, "KIND: family > class > leaf") || strings.Contains(p.Kind, "changelog") {
		t.Fatalf("kind prompt: %s", p.Kind)
	}
	if strings.Contains(p.Kind, "3 to 5") || strings.Contains(p.Kind, "one or two specific leaves") {
		t.Fatalf("kind prompt still allows extra levels: %s", p.Kind)
	}
	if EssencePromptsHash(p) == EssencePromptHash(DefaultEssenceSystemPrompt()) {
		t.Fatal("triple hash must differ from legacy single prompt hash")
	}
}
