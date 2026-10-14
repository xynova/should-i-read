package mailreport

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/behaviorengineering/strop/pkg/openaibatch"
	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
)

type fakeTldrBatch struct {
	calls int
}

func (f *fakeTldrBatch) RunChatLines(_ context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error) {
	f.calls++
	out := make(map[string]openaibatch.LineResult, len(lines))
	for _, ln := range lines {
		out[ln.CustomID] = openaibatch.LineResult{Content: "Weekly Fabric releases."}
	}
	return out, nil
}

type fakeTldrBatchErr struct{}

func (f *fakeTldrBatchErr) RunChatLines(_ context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error) {
	out := make(map[string]openaibatch.LineResult, len(lines))
	for _, ln := range lines {
		out[ln.CustomID] = openaibatch.LineResult{ErrMessage: "chat failed"}
	}
	return out, nil
}

func testClusterRunner(t *testing.T, batch ChatLineRunner, terms []string) *Runner {
	st := testLedger(t)
	cat, err := catalog.BuildCatalog(testInboxVocab())
	if err != nil {
		t.Fatal(err)
	}
	h, err := harness.CreateHarness(harness.Config{
		Judge:  &stubJudge{},
		Author: &stubAuthor{},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := CreateRunner(RunnerConfig{
		Harness:          h,
		Store:            st,
		ClassifyMax:      50,
		TldrBatch:        batch,
		TldrModel:        "chat-model",
		ClusterMinCosine: 0.8,
		QuarantineTerms:  terms,
		Now:              time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}, testInboxVocab(), cat)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func clusterCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestClusterSkipsChatSizeOne(t *testing.T) {
	batch := &fakeTldrBatch{}
	r := testClusterRunner(t, batch, nil)
	ctx := clusterCtx(t)
	art, err := r.Cluster(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "h1", CanonicalTermID: "changelog", About: "About one", Subject: "Subj"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.calls != 0 {
		t.Fatalf("calls: %d", batch.calls)
	}
	if len(art.Clusters) != 1 || art.Clusters[0].TLDR != "About one" {
		t.Fatalf("clusters: %+v", art.Clusters)
	}
}

func TestClusterSkipsChatFingerprintHit(t *testing.T) {
	st, err := hoststore.CreateStore(hoststore.StoreConfig{
		Path: filepath.Join(t.TempDir(), "ledger.db"),
		Now:  time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := clusterCtx(t)
	rows := []MessageRow{
		{ObjectHash: "h1", CanonicalTermID: "changelog"},
		{ObjectHash: "h2", CanonicalTermID: "changelog"},
	}
	_, clusters, err := AssignClusters(ctx, rows, AssignOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatal("expected one cluster")
	}
	if err := st.PutCluster(ctx, hoststore.ClusterRecord{
		ClusterID:       clusters[0].ID,
		Fingerprint:     clusters[0].Fingerprint,
		TermID:          "changelog",
		Size:            2,
		TLDR:            "Cached summary.",
		SuggestedAction: SuggestedReview,
		TLDRModel:       "m",
		MemberHashes:    clusters[0].MemberHashes,
	}); err != nil {
		t.Fatal(err)
	}
	batch := &fakeTldrBatch{}
	cat, err := catalog.BuildCatalog(testInboxVocab())
	if err != nil {
		t.Fatal(err)
	}
	h, err := harness.CreateHarness(harness.Config{Judge: &stubJudge{}, Author: &stubAuthor{}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := CreateRunner(RunnerConfig{
		Harness: h, Store: st, TldrBatch: batch, TldrModel: "m", Now: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}, testInboxVocab(), cat)
	if err != nil {
		t.Fatal(err)
	}
	art, err := r.Cluster(ctx, Artifact{Messages: rows})
	if err != nil {
		t.Fatal(err)
	}
	if batch.calls != 0 {
		t.Fatalf("calls: %d", batch.calls)
	}
	if art.Clusters[0].TLDR != "Cached summary." {
		t.Fatalf("tldr: %q", art.Clusters[0].TLDR)
	}
}

func TestClusterChatOnMiss(t *testing.T) {
	batch := &fakeTldrBatch{}
	r := testClusterRunner(t, batch, nil)
	ctx := clusterCtx(t)
	art, err := r.Cluster(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "h1", CanonicalTermID: "changelog", Subject: "A"},
			{ObjectHash: "h2", CanonicalTermID: "changelog", Subject: "B"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.calls != 1 {
		t.Fatalf("calls: %d", batch.calls)
	}
	if art.Clusters[0].TLDR != "Weekly Fabric releases." {
		t.Fatalf("tldr: %q", art.Clusters[0].TLDR)
	}
}

func TestClusterChatErrorSetsTLDRError(t *testing.T) {
	r := testClusterRunner(t, &fakeTldrBatchErr{}, nil)
	ctx := clusterCtx(t)
	art, err := r.Cluster(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "h1", CanonicalTermID: "changelog", TermID: "changelog"},
			{ObjectHash: "h2", CanonicalTermID: "changelog", TermID: "changelog"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Messages[0].TermID != "changelog" {
		t.Fatalf("term lost: %q", art.Messages[0].TermID)
	}
	if art.Clusters[0].TLDRError == "" {
		t.Fatal("expected tldr_error")
	}
}

func TestClusterQuarantineTerms(t *testing.T) {
	batch := &fakeTldrBatch{}
	r := testClusterRunner(t, batch, []string{"changelog"})
	ctx := clusterCtx(t)
	art, err := r.Cluster(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "h1", CanonicalTermID: "changelog"},
			{ObjectHash: "h2", CanonicalTermID: "changelog"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Clusters[0].SuggestedAction != SuggestedQuarantineCandidate {
		t.Fatalf("action: %q", art.Clusters[0].SuggestedAction)
	}
}

func TestClusterNilTldrRunnerDoesNotFail(t *testing.T) {
	r := testClusterRunner(t, nil, nil)
	ctx := clusterCtx(t)
	art, err := r.Cluster(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "h1", CanonicalTermID: "changelog"},
			{ObjectHash: "h2", CanonicalTermID: "changelog"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Clusters[0].TLDRError == "" {
		t.Fatal("expected tldr_error")
	}
}

func TestClusterChatBatchErrorKeepsClusters(t *testing.T) {
	r := testClusterRunner(t, &fakeTldrBatchFail{}, nil)
	ctx := clusterCtx(t)
	art, err := r.Cluster(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "h1", CanonicalTermID: "changelog"},
			{ObjectHash: "h2", CanonicalTermID: "changelog"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(art.Clusters) != 1 {
		t.Fatalf("clusters: %+v", art.Clusters)
	}
}

type fakeTldrBatchFail struct{}

func (f *fakeTldrBatchFail) RunChatLines(_ context.Context, _ []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error) {
	return nil, errors.New("batch down")
}

func TestClusterRequiresDeadline(t *testing.T) {
	r := testClusterRunner(t, nil, nil)
	_, err := r.Cluster(context.Background(), Artifact{})
	if err == nil {
		t.Fatal("expected error")
	}
}
