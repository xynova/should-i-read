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

func TestValidateAssignmentPath_rejectsDeepCatalogPath(t *testing.T) {
	t.Parallel()
	err := validateAssignmentPath(MessageRow{
		Kind: "notification > ci > pull-request",
		Path: []string{"notification", "ci", "pull-request", "preview-removal"},
	})
	if err == nil || !strings.Contains(err.Error(), AssignmentErrPathDeep) {
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

func TestAlignAssignmentPathWithKind_hyphenatedClass(t *testing.T) {
	t.Parallel()
	row := MessageRow{
		TermID: "aws",
		Kind:   "notification > job-alert > aws",
		Path:   []string{"notification", "notification-job-alert"},
	}
	alignAssignmentPathWithKind(&row)
	if err := validateAssignmentPath(row); err != nil {
		t.Fatalf("validate: %v", err)
	}
	want := []string{"notification", "job-alert", "aws"}
	if len(row.Path) != 3 || row.Path[0] != want[0] || row.Path[1] != want[1] || row.Path[2] != want[2] {
		t.Fatalf("path=%v want %v", row.Path, want)
	}
}

func TestAlignAssignmentPathWithKind_hyphenatedClassAlreadyThree(t *testing.T) {
	t.Parallel()
	row := MessageRow{
		TermID: "aws",
		Kind:   "notification > job-alert > aws",
		Path:   []string{"notification", "notification-job-alert", "aws"},
	}
	alignAssignmentPathWithKind(&row)
	if err := validateAssignmentPath(row); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if row.Path[1] != "job-alert" {
		t.Fatalf("path=%v", row.Path)
	}
}

func TestAlignAssignmentPathWithKind_emptyBreadcrumbUsesKind(t *testing.T) {
	t.Parallel()
	row := MessageRow{
		TermID: "design-review",
		Kind:   "event > meeting > design-review",
	}
	alignAssignmentPathWithKind(&row)
	if err := validateAssignmentPath(row); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(row.Path) != 3 || row.Path[2] != "design-review" {
		t.Fatalf("path=%v", row.Path)
	}
}

func TestAlignAssignmentPathWithKind_clipsDeepCatalogPrefix(t *testing.T) {
	t.Parallel()
	row := MessageRow{
		TermID: "notification-ci-pull-request-preview-removal",
		Kind:   "notification > ci > pull-request",
		Path:   []string{"notification", "ci", "pull-request", "preview-removal"},
	}
	alignAssignmentPathWithKind(&row)
	if err := validateAssignmentPath(row); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(row.Path) != 3 || row.Path[2] != "pull-request" {
		t.Fatalf("path=%v", row.Path)
	}
}

func TestClipCatalogTermToKindDepth_walksToThreeLevelParent(t *testing.T) {
	t.Parallel()
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "test",
		Terms: []catalog.Term{
			{ID: "notification", Label: "Notification", Segment: "notification"},
			{ID: "notification-ci", Label: "CI", Parent: "notification", Segment: "ci"},
			{ID: "notification-ci-pull-request", Label: "Pull Request", Parent: "notification-ci", Segment: "pull-request"},
			{ID: "notification-ci-pull-request-preview-removal", Label: "Preview Removal", Parent: "notification-ci-pull-request", Segment: "preview-removal"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	row := MessageRow{
		TermID: "notification-ci-pull-request-preview-removal",
		Kind:   "notification > ci > pull-request",
		Path:   []string{"notification", "ci", "pull-request", "preview-removal"},
	}
	alignAssignmentPathWithKind(&row)
	clipCatalogTermToKindDepth(cat, &row)
	if err := validateAssignmentPath(row); err != nil {
		t.Fatalf("got %v", err)
	}
	if row.TermID != "notification-ci-pull-request" {
		t.Fatalf("term %q", row.TermID)
	}
	if len(row.Path) != 3 || row.Path[2] != "pull-request" {
		t.Fatalf("path=%v", row.Path)
	}
}

func TestAlignAssignmentPathWithKind_classMismatchUnchanged(t *testing.T) {
	t.Parallel()
	row := MessageRow{
		TermID: "aws",
		Kind:   "notification > job-alert > aws",
		Path:   []string{"notification", "billing"},
	}
	before := append([]string{}, row.Path...)
	alignAssignmentPathWithKind(&row)
	if row.Path[0] != before[0] || row.Path[1] != before[1] {
		t.Fatalf("path changed: %v", row.Path)
	}
	err := validateAssignmentPath(row)
	if err == nil || !strings.Contains(err.Error(), AssignmentErrPathShort) {
		t.Fatalf("got %v", err)
	}
}
