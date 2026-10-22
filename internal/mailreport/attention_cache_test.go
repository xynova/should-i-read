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

func TestAttentionInputHash_stableForSameFields(t *testing.T) {
	in := AttentionIn{Subject: "hi", Sender: "a@b.c", Kind: "k", About: "about"}
	h1 := AttentionInputHash(in)
	h2 := AttentionInputHash(in)
	if h1 == "" || h1 != h2 {
		t.Fatalf("h1=%q h2=%q", h1, h2)
	}
}

func TestAttentionInputHash_kindChangeMisses(t *testing.T) {
	base := AttentionIn{Subject: "hi", Sender: "a@b.c", Kind: "k1", About: "about"}
	other := AttentionIn{Subject: "hi", Sender: "a@b.c", Kind: "k2", About: "about"}
	if AttentionInputHash(base) == AttentionInputHash(other) {
		t.Fatal("expected different hashes when kind changes")
	}
}

func TestScoreAttention_duplicateInputOneSystemOne(t *testing.T) {
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
		JudgeModel:       "judge-test",
		Attention:        att,
		AttentionTrueMin: 0.5,
		Now:              time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}, testInboxVocab(), cat)
	if err != nil {
		t.Fatal(err)
	}
	same := AttentionIn{Subject: "subj", Sender: "s@x.com", Kind: "kind", About: "about"}
	for _, hash := range []string{"hash-a", "hash-b"} {
		tok, _, err := st.Claim(ctx, hash)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Complete(ctx, hash, tok, hoststore.StateRow{TermID: "t"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	art := Artifact{
		Messages: []MessageRow{
			{ObjectHash: "hash-a", Subject: same.Subject, Sender: same.Sender, Kind: same.Kind, About: same.About},
			{ObjectHash: "hash-b", Subject: same.Subject, Sender: same.Sender, Kind: same.Kind, About: same.About},
		},
	}
	art, err = r.ScoreAttention(ctx, art)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("stub calls=%d want 1", calls.Load())
	}
	if art.AttentionScored != 2 {
		t.Fatalf("scored=%d want 2", art.AttentionScored)
	}
	for i := range art.Messages {
		if art.Messages[i].Attention == nil {
			t.Fatalf("row %d missing attention", i)
		}
	}
}

func TestScoreAttention_cacheHitStillPutAttention(t *testing.T) {
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
	in := AttentionIn{Subject: "subj", Sender: "s@x.com", Kind: "kind", About: "about"}
	inputHash := AttentionInputHash(in)
	promptHash := AttentionPromptHash()
	judgeModel := "judge-test"
	if err := st.PutAttentionInput(ctx, inputHash, judgeModel, promptHash, 0.5, 0.2, 0.3); err != nil {
		t.Fatal(err)
	}
	tok, _, err := st.Claim(ctx, "hash-cache")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "hash-cache", tok, hoststore.StateRow{TermID: "t"}, nil); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	att := &stubAttentionClient{fn: func(_ context.Context, _ AttentionIn) (Attention, error) {
		calls.Add(1)
		return Attention{Urgency: 0.9}, nil
	}}
	r, err := CreateRunner(RunnerConfig{
		Harness:          h,
		Store:            st,
		ClassifyMax:      50,
		JudgeModel:       judgeModel,
		Attention:        att,
		AttentionTrueMin: 0.5,
		Now:              time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}, testInboxVocab(), cat)
	if err != nil {
		t.Fatal(err)
	}
	art := Artifact{
		Messages: []MessageRow{
			{ObjectHash: "hash-cache", Subject: in.Subject, Sender: in.Sender, Kind: in.Kind, About: in.About},
		},
	}
	if _, err = r.ScoreAttention(ctx, art); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatalf("expected cache hit, stub calls=%d", calls.Load())
	}
	got, err := st.Get(ctx, "hash-cache")
	if err != nil {
		t.Fatal(err)
	}
	if !got.AttentionScored || got.Urgency != 0.5 {
		t.Fatalf("expected PutAttention on object_hash, got %+v", got)
	}
}
