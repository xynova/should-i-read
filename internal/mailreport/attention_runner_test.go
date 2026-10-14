package mailreport

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
)

func TestScoreAttentionResume(t *testing.T) {
	st := testLedger(t)
	ctx := clusterCtx(t)
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
	var calls atomic.Int32
	att := &stubAttentionClient{fn: func(_ context.Context, _ AttentionIn) (Attention, error) {
		calls.Add(1)
		return Attention{Urgency: 0.42, NeedsReplyNoul: 0.1, InvolvesDecisionNoul: 0.6}, nil
	}}
	r, err := CreateRunner(RunnerConfig{
		Harness:          h,
		Store:            st,
		ClassifyMax:      50,
		Attention:        att,
		AttentionTrueMin: 0.5,
		Now:              time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}, testInboxVocab(), cat)
	if err != nil {
		t.Fatal(err)
	}
	art := Artifact{
		Messages: []MessageRow{{
			ObjectHash: "hash-att",
			Subject:    "hello",
			About:      "world",
		}},
	}
	tok, _, err := st.Claim(ctx, "hash-att")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "hash-att", tok, hoststore.StateRow{TermID: "t"}, nil); err != nil {
		t.Fatal(err)
	}
	art, err = r.ScoreAttention(ctx, art)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || art.AttentionScored != 1 {
		t.Fatalf("calls=%d scored=%d", calls.Load(), art.AttentionScored)
	}
	art, err = r.ScoreAttention(ctx, art)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected no second score call, got %d", calls.Load())
	}
}
