package hoststore

import (
	"testing"
)

func TestPutGetAttentionInput_roundTrip(t *testing.T) {
	st, ctx := testStore(t)
	inHash := "abc123"
	model := "judge-m"
	prompt := "prompt-hash"
	if err := st.PutAttentionInput(ctx, inHash, model, prompt, 0.42, 0.1, 0.6); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetAttentionInput(ctx, inHash, model, prompt)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Urgency != 0.42 || got.NeedsReplyNoul != 0.1 || got.InvolvesDecisionNoul != 0.6 {
		t.Fatalf("got %+v", got)
	}
}

func TestGetAttentionInput_miss(t *testing.T) {
	st, ctx := testStore(t)
	got, err := st.GetAttentionInput(ctx, "missing", "m", "p")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
