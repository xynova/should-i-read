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

func testAttentionShapeRunner(t *testing.T, st *hoststore.Store, att AttentionClient, classifyMax int) *Runner {
	t.Helper()
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
	max := classifyMax
	if max <= 0 {
		max = 50
	}
	r, err := CreateRunner(RunnerConfig{
		Harness:            h,
		Store:              st,
		ClassifyMax:        max,
		JudgeModel:         "judge-test",
		Attention:          att,
		AttentionTrueMin:   0.5,
		ShapeAllowedLeaves: []string{"pipeline-failure"},
		Now:                time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}, testInboxVocab(), cat)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func completeHash(t *testing.T, st *hoststore.Store, ctx context.Context, hash string) {
	t.Helper()
	tok, _, err := st.Claim(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, hash, tok, hoststore.StateRow{TermID: "t"}, nil); err != nil {
		t.Fatal(err)
	}
}

func insertActivePipelineShape(t *testing.T, r *Runner, st *hoststore.Store, ctx context.Context, seed, subject, sender string) string {
	t.Helper()
	_, _, fp, ok := r.shapeFingerprintForItem(Item{Subject: subject, Sender: sender})
	if !ok || fp == "" {
		t.Fatalf("fingerprint ok=%v fp=%q", ok, fp)
	}
	if _, err := st.RecordShapeHit(ctx, hoststore.ShapeHit{
		Fingerprint:    fp,
		Domain:         senderDomain(sender),
		Template:       r.subjectTemplater.Template(subject),
		Kind:           "notification > ci > pipeline-failure",
		TermID:         "pipeline-failure",
		LastSlot:       "pipeline-failure",
		PromptHash:     r.currentEssencePromptHash(),
		SeedObjectHash: seed,
		MinRepeats:     1,
	}); err != nil {
		t.Fatal(err)
	}
	return fp
}

func TestScoreAttention_shapeFingerprintReusesSystemOne(t *testing.T) {
	st := testLedger(t)
	ctx := clusterCtx(t)
	var calls atomic.Int32
	att := &stubAttentionClient{fn: func(_ context.Context, _ AttentionIn) (Attention, error) {
		calls.Add(1)
		return Attention{Urgency: 0.31, NeedsReplyNoul: 0.02, InvolvesDecisionNoul: 0.04}, nil
	}}
	r := testAttentionShapeRunner(t, st, att, 50)
	sender := "GitLab <notify@gitlab.com>"
	first := "Acme | Failed pipeline for main | deadbeef"
	second := "Acme | Failed pipeline for main | cafebabe"
	completeHash(t, st, ctx, "hash-a")
	completeHash(t, st, ctx, "hash-b")
	insertActivePipelineShape(t, r, st, ctx, "hash-a", first, sender)

	art, err := r.ScoreAttention(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "hash-a", Subject: first, Sender: sender, Kind: "notification > ci > pipeline-failure", About: first},
			{ObjectHash: "hash-b", Subject: second, Sender: sender, Kind: "notification > ci > pipeline-failure", About: second},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("stub calls=%d want 1", calls.Load())
	}
	if art.AttentionScored != 2 || art.AttentionShapeReused != 1 {
		t.Fatalf("scored=%d reused=%d", art.AttentionScored, art.AttentionShapeReused)
	}
	if art.Messages[1].Attention == nil || art.Messages[1].Attention.Urgency != 0.31 {
		t.Fatalf("copy attention=%+v", art.Messages[1].Attention)
	}
}

func TestScoreAttention_shapeSeedBackfillSkipsSystemOne(t *testing.T) {
	st := testLedger(t)
	ctx := clusterCtx(t)
	var calls atomic.Int32
	att := &stubAttentionClient{fn: func(_ context.Context, _ AttentionIn) (Attention, error) {
		calls.Add(1)
		return Attention{Urgency: 0.9}, nil
	}}
	r := testAttentionShapeRunner(t, st, att, 50)
	sender := "GitLab <notify@gitlab.com>"
	seedSubj := "Acme | Failed pipeline for main | deadbeef"
	copySubj := "Acme | Failed pipeline for main | 111aaaa"
	completeHash(t, st, ctx, "seed-hash")
	completeHash(t, st, ctx, "copy-hash")
	if err := st.PutAttention(ctx, "seed-hash", 0.22, 0.01, 0.03); err != nil {
		t.Fatal(err)
	}
	insertActivePipelineShape(t, r, st, ctx, "seed-hash", seedSubj, sender)

	art, err := r.ScoreAttention(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "copy-hash", Subject: copySubj, Sender: sender, Kind: "notification > ci > pipeline-failure", About: copySubj},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatalf("expected seed backfill, stub calls=%d", calls.Load())
	}
	if art.AttentionShapeReused != 1 || art.AttentionScored != 1 {
		t.Fatalf("scored=%d reused=%d", art.AttentionScored, art.AttentionShapeReused)
	}
	got, err := st.Get(ctx, "copy-hash")
	if err != nil {
		t.Fatal(err)
	}
	if !got.AttentionScored || got.Urgency != 0.22 {
		t.Fatalf("copy state=%+v", got)
	}
}

func TestScoreAttention_shapeReuseIgnoresEssencePromptHash(t *testing.T) {
	st := testLedger(t)
	ctx := clusterCtx(t)
	var calls atomic.Int32
	att := &stubAttentionClient{fn: func(_ context.Context, _ AttentionIn) (Attention, error) {
		calls.Add(1)
		return Attention{Urgency: 0.9}, nil
	}}
	r := testAttentionShapeRunner(t, st, att, 50)
	sender := "GitLab <notify@gitlab.com>"
	subj := "Acme | Failed pipeline for main | deadbeef"
	completeHash(t, st, ctx, "copy-ph")
	_, _, fp, ok := r.shapeFingerprintForItem(Item{Subject: subj, Sender: sender})
	if !ok {
		t.Fatal("fingerprint")
	}
	if _, err := st.RecordShapeHit(ctx, hoststore.ShapeHit{
		Fingerprint:    fp,
		Domain:         senderDomain(sender),
		Template:       r.subjectTemplater.Template(subj),
		Kind:           "notification > ci > pipeline-failure",
		TermID:         "pipeline-failure",
		LastSlot:       "pipeline-failure",
		PromptHash:     "stale-essence-prompt",
		SeedObjectHash: "seed-ph",
		MinRepeats:     1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutShapeAttention(ctx, fp, 0.33, 0.0, 0.0); err != nil {
		t.Fatal(err)
	}
	art, err := r.ScoreAttention(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "copy-ph", Subject: subj, Sender: sender, Kind: "notification > ci > pipeline-failure", About: subj},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || art.AttentionShapeReused != 1 {
		t.Fatalf("calls=%d reused=%d", calls.Load(), art.AttentionShapeReused)
	}
}

func TestScoreAttention_shapeReuseIgnoresHopCap(t *testing.T) {
	st := testLedger(t)
	ctx := clusterCtx(t)
	var calls atomic.Int32
	att := &stubAttentionClient{fn: func(_ context.Context, _ AttentionIn) (Attention, error) {
		calls.Add(1)
		return Attention{Urgency: 0.5}, nil
	}}
	r := testAttentionShapeRunner(t, st, att, 1)
	r.aiHops = 1
	sender := "GitLab <notify@gitlab.com>"
	subj := "Acme | Failed pipeline for main | deadbeef"
	completeHash(t, st, ctx, "copy-cap")
	fp := insertActivePipelineShape(t, r, st, ctx, "seed-cap", subj, sender)
	if err := st.PutShapeAttention(ctx, fp, 0.41, 0.0, 0.0); err != nil {
		t.Fatal(err)
	}
	art, err := r.ScoreAttention(ctx, Artifact{
		Messages: []MessageRow{
			{ObjectHash: "copy-cap", Subject: subj, Sender: sender, Kind: "notification > ci > pipeline-failure", About: subj},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || art.AttentionOmitted != 0 || art.AttentionShapeReused != 1 {
		t.Fatalf("calls=%d omitted=%d reused=%d", calls.Load(), art.AttentionOmitted, art.AttentionShapeReused)
	}
}
