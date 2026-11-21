package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"github.com/xynova/should-i-read/internal/hoststore"
)

func worthReadRunner(t *testing.T, st *hoststore.Store) *Runner {
	t.Helper()
	seats := Seats{Judge: &stubJudge{}, Author: &stubAuthor{}}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	vocab := testInboxVocab()
	cat := testInboxCatalog(t)
	r, err := CreateRunner(RunnerConfig{
		Harness:               h,
		Seats:                 seats,
		Store:                 st,
		ClassifyMax:           10,
		WorthReadPrior:        true,
		WorthReadMinNeighbors: 1,
		WorthReadSimilarK:     5,
		Now:                   time.Now().UTC(),
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestScoreWorthReadPriorExactTrash(t *testing.T) {
	st, ctx := openWorthStore(t)
	if err := st.UpsertOutcome(ctx, hoststore.MessageOutcome{
		ObjectHash: "h1", Outcome: hoststore.OutcomeTrashedAtSource, Source: hoststore.OutcomeSourceDeleteJob,
	}); err != nil {
		t.Fatal(err)
	}
	r := worthReadRunner(t, st)
	art, err := r.ScoreWorthReadPrior(ctx, Artifact{
		Mode:     "report_only",
		Messages: []MessageRow{{ObjectHash: "h1", Sender: "n@github.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Messages[0].WorthReadPrior == nil || art.Messages[0].WorthReadPrior.Score != 0 || art.Messages[0].WorthReadPrior.Basis != "exact" {
		t.Fatalf("%+v", art.Messages[0].WorthReadPrior)
	}
}

func TestScoreWorthReadPriorNeighbors(t *testing.T) {
	st, ctx := openWorthStore(t)
	model := "embed-1"
	for i, h := range []string{"a", "b", "c", "target"} {
		vec := []float64{1, 0, 0}
		if i > 0 {
			vec = []float64{0.99, 0.01, 0}
		}
		if err := st.PutEmbedding(ctx, h, model, vec); err != nil {
			t.Fatal(err)
		}
		if h != "target" {
			if err := st.UpsertOutcome(ctx, hoststore.MessageOutcome{
				ObjectHash: h, Outcome: hoststore.OutcomeTrashedAtSource, Source: hoststore.OutcomeSourceDeleteJob,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	r := worthReadRunner(t, st)
	r.embedModel = model
	r.attachMinCosine = 0.5
	art, err := r.ScoreWorthReadPrior(ctx, Artifact{
		Mode:       "report_only",
		EmbedModel: model,
		Messages:   []MessageRow{{ObjectHash: "target", Sender: "n@github.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := art.Messages[0].WorthReadPrior
	if p == nil || p.Basis != "neighbors" || p.Score != 0 {
		t.Fatalf("%+v", p)
	}
}

func TestScoreWorthReadPriorOmitsWhenEmpty(t *testing.T) {
	st, ctx := openWorthStore(t)
	r := worthReadRunner(t, st)
	art, err := r.ScoreWorthReadPrior(ctx, Artifact{
		Mode:     "report_only",
		Messages: []MessageRow{{ObjectHash: "h1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Messages[0].WorthReadPrior != nil {
		t.Fatalf("%+v", art.Messages[0].WorthReadPrior)
	}
}

func openWorthStore(t *testing.T) (*hoststore.Store, context.Context) {
	t.Helper()
	path := t.TempDir() + "/ledger.db"
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = st.Close() })
	return st, ctx
}
