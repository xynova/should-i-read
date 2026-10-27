package mailreport

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

func TestBuildAuthorPrompt_listsLeafIds(t *testing.T) {
	cat := testInboxCatalog(t)
	leafLines := formatAuthorLeaves(cat)
	in := harness.DraftIn{
		WorldContext: WorldContext,
		Text:         "Subject: T\nFrom: x@y.com\n\nBody",
		Reason:       "judge skip",
		Parents:      []harness.PackedOption{{ParentID: "unwanted", Label: "Unwanted"}},
	}
	prompt := buildAuthorPrompt(in, leafLines, AuthorMessageBody(in.Text))
	if !strings.Contains(prompt, "Allowed leaf ids") {
		t.Fatal("want allowed leaf ids section")
	}
	if !strings.Contains(prompt, "id=notification") {
		t.Fatalf("want catalog leaf in prompt: %s", prompt)
	}
	if strings.Contains(prompt, "Classify this email") {
		t.Fatal("want constrained prompt, not open classify")
	}
}

func TestValidateAuthorDraft_rejectsUnknownLeaf(t *testing.T) {
	cat := testInboxCatalog(t)
	err := validateAuthorDraft(cat, nil, authorJSON{
		Kind:   harness.DraftKindAlias,
		LeafID: "not-a-real-leaf",
		Alias:  "foo",
	})
	if err == nil {
		t.Fatal("want error for unknown leafId")
	}
}

func TestValidateAuthorDraft_rejectsRectangularDescription(t *testing.T) {
	cat := testInboxCatalog(t)
	err := validateAuthorDraft(cat, []harness.PackedOption{{ParentID: "unwanted", Label: "Unwanted"}}, authorJSON{
		Kind:        harness.DraftKindNewLeaf,
		ID:          "font",
		Parent:      "unwanted",
		Label:       "Font",
		Description: "Rectangular",
	})
	if err == nil {
		t.Fatal("want description error")
	}
}

func TestValidateAuthorDraft_rejectsParentPrefixedID(t *testing.T) {
	cat := testInboxCatalog(t)
	err := validateAuthorDraft(cat, []harness.PackedOption{{ParentID: "notification", Label: "Notification"}}, authorJSON{
		Kind:        harness.DraftKindNewLeaf,
		ID:          "notification-ci",
		Parent:      "notification",
		Label:       "CI",
		Description: "GitHub Actions workflow failure notifications for a repository.",
	})
	if err == nil {
		t.Fatal("want parent-prefix id error")
	}
}

func TestValidateBreadcrumbCatalogInput_rejectsLayoutAbout(t *testing.T) {
	t.Parallel()
	err := validateBreadcrumbCatalogInput("Rectangular", "notification ci")
	if err == nil {
		t.Fatal("want error for layout about")
	}
}

func TestValidateBreadcrumbCatalogInput_kebabShape(t *testing.T) {
	t.Parallel()
	about := "Bridget Shelton sent the executed agreement for the commission case."
	if err := validateBreadcrumbCatalogInput(about, "legal-notice"); err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestValidateCatalogShape_layoutBeforeKebab(t *testing.T) {
	t.Parallel()
	err := validateCatalogShape("rectangular")
	if err == nil || !strings.Contains(err.Error(), CatalogErrDescriptionLayout) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateBreadcrumbCatalogInput_aboutStillNeedsFourWords(t *testing.T) {
	t.Parallel()
	err := validateBreadcrumbCatalogInput("too few words", "legal-notice")
	if err == nil || !strings.Contains(err.Error(), CatalogErrDescriptionShort) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateBreadcrumbCatalogInput_acceptsShapeTagLine(t *testing.T) {
	t.Parallel()
	err := validateBreadcrumbCatalogInput(
		"This message is a notification about a failed GitHub Actions workflow run.",
		"notification ci",
	)
	if err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestValidateAuthorDraft_acceptsGroundedNewLeaf(t *testing.T) {
	cat := testInboxCatalog(t)
	err := validateAuthorDraft(cat, []harness.PackedOption{{ParentID: "unwanted", Label: "Unwanted"}}, authorJSON{
		Kind:        harness.DraftKindNewLeaf,
		ID:          "receipt",
		Parent:      "unwanted",
		Label:       "Receipt",
		Description: "A payment receipt for a completed subscription charge.",
	})
	if err != nil {
		t.Fatalf("want valid new_leaf: %v", err)
	}
}

func TestValidateAuthorDraft_acceptsAlias(t *testing.T) {
	cat := testInboxCatalog(t)
	err := validateAuthorDraft(cat, []harness.PackedOption{{ParentID: "unwanted", Label: "Unwanted"}}, authorJSON{
		Kind:   harness.DraftKindAlias,
		LeafID: "notification",
		Alias:  "github-alert",
	})
	if err != nil {
		t.Fatalf("want valid alias: %v", err)
	}
}
