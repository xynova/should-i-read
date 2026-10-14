package hoststore

import (
	"context"
	"testing"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestPutAttentionCompleteGet(t *testing.T) {
	st, ctx := testStore(t)
	tok, _, err := st.Claim(ctx, "att1")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "att1", tok, StateRow{TermID: "reminder"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.PutAttention(ctx, "att1", 0.82, 0.1, 0.91); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(ctx, "att1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.AttentionScored || got.Urgency != 0.82 || got.NeedsReplyNoul != 0.1 || got.InvolvesDecisionNoul != 0.91 {
		t.Fatalf("got %+v", got)
	}
}

func TestPutAttentionNotComplete(t *testing.T) {
	st, ctx := testStore(t)
	tok, _, err := st.Claim(ctx, "att2")
	if err != nil {
		t.Fatal(err)
	}
	err = st.PutAttention(ctx, "att2", 0.5, 0.5, 0.5)
	if err == nil {
		t.Fatal("expected error on running row")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
	_ = st.Fail(ctx, "att2", tok, "fail")
}

func TestCompleteResetsAttention(t *testing.T) {
	st, ctx := testStore(t)
	tok, _, err := st.Claim(ctx, "att3")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "att3", tok, StateRow{TermID: "failure"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.PutAttention(ctx, "att3", 0.45, 0.05, 0.55); err != nil {
		t.Fatal(err)
	}
	tok2 := "forced-reclaim-token"
	_, err = st.db.ExecContext(ctx, `
UPDATE classify_state SET status='running', claim_token=? WHERE object_hash=?`, tok2, "att3")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "att3", tok2, StateRow{TermID: "failure", Kind: "k"}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(ctx, "att3")
	if err != nil {
		t.Fatal(err)
	}
	if got.AttentionScored || got.Urgency != 0 {
		t.Fatalf("attention not reset: %+v", got)
	}
}

func TestPutClusterAttentionFields(t *testing.T) {
	st, ctx := testStore(t)
	rec := sampleCluster()
	rec.Urgency = 0.82
	rec.NeedsReply = true
	rec.InvolvesDecision = false
	rec.AttentionScored = true
	if err := st.PutCluster(ctx, rec); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetClusterByFingerprint(ctx, rec.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if got.Urgency != 0.82 || !got.NeedsReply || got.InvolvesDecision || !got.AttentionScored {
		t.Fatalf("got %+v", got)
	}
}

func TestPutAttentionRequiresDeadline(t *testing.T) {
	st, _ := testStore(t)
	err := st.PutAttention(context.Background(), "x", 0, 0, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
