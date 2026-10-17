package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

func TestMergeClusters_unionsSliceTLDRByFingerprint(t *testing.T) {
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	term := "newsletter"
	gk := "term:" + term
	id := clusterIDFromGroupKey(gk)
	fp := clusterFingerprint(gk, []string{"a", "b"})
	sliceClusters := [][]ClusterRow{{
		{
			ID:           id,
			TermID:       term,
			Size:         2,
			Fingerprint:  fp,
			TLDR:         "cached tldr",
			MemberHashes: []string{"a", "b"},
		},
	}}
	rows := []MessageRow{
		{ObjectHash: "a", TermID: term, CanonicalTermID: term},
		{ObjectHash: "b", TermID: term, CanonicalTermID: term},
		{ObjectHash: "c", TermID: term, CanonicalTermID: term},
	}
	runner, err := CreateRunner(RunnerConfig{
		Harness: mustWalkHarness(t),
		Seats:   Seats{Judge: &stubJudge{}, Author: &stubAuthor{}},
		Store:   store,
		Now:     time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}, testInboxVocab(), testInboxCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	outRows, clusters, err := runner.MergeClusters(ctx, rows, sliceClusters)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("clusters=%d", len(clusters))
	}
	if clusters[0].TLDR != "cached tldr" {
		t.Fatalf("tldr=%q", clusters[0].TLDR)
	}
	if len(clusters[0].MemberHashes) != 3 {
		t.Fatalf("members=%v", clusters[0].MemberHashes)
	}
	_ = outRows
}

func mustWalkHarness(t *testing.T) *harness.Harness {
	h, err := harness.CreateHarness(harness.Config{
		Judge:  &stubJudge{},
		Author: &stubAuthor{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
