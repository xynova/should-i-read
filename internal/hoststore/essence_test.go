package hoststore

import (
	"context"
	"testing"
)

func TestEssenceFingerprintHitAndMiss(t *testing.T) {
	st, ctx := testStore(t)
	fp := EssenceFingerprint{EssenceModel: "m1", EscalateModel: "m2", PromptHash: "ph1"}
	row := EssenceRow{Kind: "a > b > c", About: "about", Shape: "tag1", Why: "why"}
	if err := st.PutEssence(ctx, "hash-a", fp, row); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetEssence(ctx, "hash-a", fp)
	if err != nil || !ok {
		t.Fatalf("GetEssence hit: ok=%v err=%v", ok, err)
	}
	if got.About != "about" {
		t.Fatalf("about=%q", got.About)
	}
	_, ok, err = st.GetEssence(ctx, "hash-a", EssenceFingerprint{EssenceModel: "other", EscalateModel: "m2", PromptHash: "ph1"})
	if err != nil || ok {
		t.Fatalf("model miss: ok=%v err=%v", ok, err)
	}
	_, ok, err = st.GetEssence(ctx, "hash-a", EssenceFingerprint{EssenceModel: "m1", EscalateModel: "m2", PromptHash: "other"})
	if err != nil || ok {
		t.Fatalf("prompt miss: ok=%v err=%v", ok, err)
	}
}

func TestEssenceMethodsRequireDeadline(t *testing.T) {
	st, _ := testStore(t)
	fp := EssenceFingerprint{EssenceModel: "m", PromptHash: "p"}
	_, _, err := st.GetEssence(context.Background(), "x", fp)
	if err == nil {
		t.Fatal("expected missing deadline")
	}
	err = st.PutEssence(context.Background(), "x", fp, EssenceRow{})
	if err == nil {
		t.Fatal("expected missing deadline")
	}
}
