package mailreport

import "testing"

func TestMergeClassifyArtifactsCopiesGeneratedAt(t *testing.T) {
	t.Parallel()
	dst := Artifact{
		WindowFrom: "2026-09-01",
		WindowTo:   "2026-10-01",
	}
	src := Artifact{
		Mode:        "report_only",
		GeneratedAt: "2026-10-18T09:00:00Z",
		Source:      SourceReport,
		CatalogID:   "inbox-kind",
		Classified:  2,
		Messages:    []MessageRow{{ObjectHash: "a"}, {ObjectHash: "b"}},
	}
	MergeClassifyArtifacts(&dst, src)
	if dst.GeneratedAt != "2026-10-18T09:00:00Z" {
		t.Fatalf("generated_at %q", dst.GeneratedAt)
	}
	if dst.Mode != "report_only" || dst.Source != SourceReport || dst.CatalogID != "inbox-kind" {
		t.Fatalf("envelope: %+v", dst)
	}
	if dst.WindowFrom != "2026-09-01" || dst.WindowTo != "2026-10-01" {
		t.Fatalf("window overwritten: %+v", dst)
	}
	if dst.Classified != 2 || len(dst.Messages) != 2 {
		t.Fatalf("rows: classified=%d n=%d", dst.Classified, len(dst.Messages))
	}

	MergeClassifyArtifacts(&dst, Artifact{
		GeneratedAt: "2026-10-18T10:00:00Z",
		Classified:  1,
		Messages:    []MessageRow{{ObjectHash: "c"}},
	})
	if dst.GeneratedAt != "2026-10-18T09:00:00Z" {
		t.Fatalf("later slice overwrote generated_at: %q", dst.GeneratedAt)
	}
	if dst.Classified != 3 || len(dst.Messages) != 3 {
		t.Fatalf("second merge: classified=%d n=%d", dst.Classified, len(dst.Messages))
	}
}
