package mailreport

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

func TestValidateAssignmentPath_walkWithoutKind(t *testing.T) {
	t.Parallel()
	if err := validateAssignmentPath(MessageRow{Path: []string{"notification"}, TermID: "notification"}); err != nil {
		t.Fatalf("walk row: %v", err)
	}
}

func TestValidateAssignmentPath_rejectsShortCatalogPath(t *testing.T) {
	t.Parallel()
	err := validateAssignmentPath(MessageRow{
		Kind: "notification > ci > pr-comment",
		Path: []string{"notification", "ci"},
	})
	if err == nil || !strings.Contains(err.Error(), AssignmentErrPathShort) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateAssignmentPath_rejectsParentPrefix(t *testing.T) {
	t.Parallel()
	err := validateAssignmentPath(MessageRow{
		Kind: "notification > ci > workflow-failure",
		Path: []string{"notification", "notification-ci", "workflow-failure"},
	})
	if err == nil || !strings.Contains(err.Error(), AssignmentErrPathAncestor) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateAssignmentPath_acceptsSemanticPath(t *testing.T) {
	t.Parallel()
	err := validateAssignmentPath(MessageRow{
		Kind: "notification > ci > workflow-failure",
		Path: []string{"notification", "ci", "workflow-failure"},
	})
	if err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestSyncRowPathFromCatalog_breadcrumbPrefix(t *testing.T) {
	t.Parallel()
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "test",
		Terms: []catalog.Term{
			{ID: "notification", Label: "Notification", Segment: "notification"},
			{ID: "blog", Label: "Blog", Parent: "notification", Segment: "blog"},
			{ID: "blog-update", Label: "Update", Parent: "blog", Segment: "update"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	row := MessageRow{
		TermID: "blog-update",
		Kind:   "notification > blog > update",
		Path:   []string{"notification", "blog"},
	}
	syncRowPathFromCatalog(cat, &row)
	if err := validateAssignmentPath(row); err != nil {
		t.Fatalf("after sync: %v", err)
	}
	if len(row.Path) != 3 || row.Path[2] != "update" {
		t.Fatalf("path=%v", row.Path)
	}
}

func TestAlignAssignmentPathWithKind_breadcrumbPrefix(t *testing.T) {
	t.Parallel()
	row := MessageRow{
		TermID: "blog-update",
		Kind:   "notification > blog > update",
		Path:   []string{"notification", "blog"},
	}
	alignAssignmentPathWithKind(&row)
	if err := validateAssignmentPath(row); err != nil {
		t.Fatalf("got %v", err)
	}
}
