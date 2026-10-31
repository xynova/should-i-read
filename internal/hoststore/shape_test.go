package hoststore

import (
	"context"
	"testing"
)

func testShapeHit(fp string, minRepeats int) ShapeHit {
	if minRepeats <= 0 {
		minRepeats = 3
	}
	return ShapeHit{
		Fingerprint:    fp,
		Domain:         "gitlab.com",
		Template:       "failed pipeline for branch | {sha}",
		Kind:           "ci > pipeline > pipeline-failure",
		TermID:         "pipeline-failure",
		LastSlot:       "pipeline-failure",
		PromptHash:     "ph-current",
		EssenceModel:   "essence-m",
		SeedObjectHash: "seed-1",
		MinRepeats:     minRepeats,
	}
}

func TestGetShapeMiss(t *testing.T) {
	st, ctx := testStore(t)
	_, ok, err := st.GetShape(ctx, "missing")
	if err != nil || ok {
		t.Fatalf("miss: ok=%v err=%v", ok, err)
	}
}

func TestRecordShapeHitFirstInsertAccumulating(t *testing.T) {
	st, ctx := testStore(t)
	res, err := st.RecordShapeHit(ctx, testShapeHit("fp-a", 3))
	if err != nil {
		t.Fatal(err)
	}
	if res.Promoted {
		t.Fatal("want not promoted on first hit")
	}
	row, ok, err := st.GetShape(ctx, "fp-a")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if row.Status != ShapeStatusAccumulating || row.HitCount != 1 {
		t.Fatalf("row=%+v", row)
	}
}

func TestRecordShapeHitPromoteAtMinRepeats(t *testing.T) {
	st, ctx := testStore(t)
	h := testShapeHit("fp-b", 3)
	for i := 0; i < 2; i++ {
		res, err := st.RecordShapeHit(ctx, h)
		if err != nil || res.Promoted {
			t.Fatalf("hit %d: promoted=%v err=%v", i+1, res.Promoted, err)
		}
	}
	res, err := st.RecordShapeHit(ctx, h)
	if err != nil || !res.Promoted {
		t.Fatalf("third hit: promoted=%v err=%v", res.Promoted, err)
	}
	row, ok, err := st.GetShape(ctx, "fp-b")
	if err != nil || !ok || row.Status != ShapeStatusActive || row.HitCount != 3 {
		t.Fatalf("row=%+v ok=%v err=%v", row, ok, err)
	}
}

func TestRecordShapeHitStalePromptHashResets(t *testing.T) {
	st, ctx := testStore(t)
	h := testShapeHit("fp-c", 3)
	if _, err := st.RecordShapeHit(ctx, h); err != nil {
		t.Fatal(err)
	}
	h2 := h
	h2.PromptHash = "ph-new"
	h2.SeedObjectHash = "seed-2"
	res, err := st.RecordShapeHit(ctx, h2)
	if err != nil || res.Promoted {
		t.Fatalf("reset: promoted=%v err=%v", res.Promoted, err)
	}
	row, ok, err := st.GetShape(ctx, "fp-c")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if row.HitCount != 1 || row.PromptHash != "ph-new" || row.ConfirmCount != 0 {
		t.Fatalf("row=%+v", row)
	}
}

func TestRecordShapeHitKindMismatchDisables(t *testing.T) {
	st, ctx := testStore(t)
	h := testShapeHit("fp-d", 3)
	if _, err := st.RecordShapeHit(ctx, h); err != nil {
		t.Fatal(err)
	}
	h2 := h
	h2.Kind = "ci > pipeline > workflow-failure"
	h2.LastSlot = "workflow-failure"
	if _, err := st.RecordShapeHit(ctx, h2); err != nil {
		t.Fatal(err)
	}
	row, ok, err := st.GetShape(ctx, "fp-d")
	if err != nil || !ok || row.Status != ShapeStatusDisabled {
		t.Fatalf("row=%+v", row)
	}
}

func TestRejectShapeDisablesAfterThree(t *testing.T) {
	st, ctx := testStore(t)
	h := testShapeHit("fp-e", 1)
	res, err := st.RecordShapeHit(ctx, h)
	if err != nil || !res.Promoted {
		t.Fatalf("promote: %v %v", res, err)
	}
	for i := 0; i < 3; i++ {
		if err := st.RejectShape(ctx, "fp-e"); err != nil {
			t.Fatal(err)
		}
	}
	row, ok, err := st.GetShape(ctx, "fp-e")
	if err != nil || !ok || row.Status != ShapeStatusDisabled {
		t.Fatalf("row=%+v", row)
	}
}

func TestShapeMethodsRequireDeadline(t *testing.T) {
	st, _ := testStore(t)
	h := testShapeHit("fp-x", 3)
	_, _, err := st.GetShape(context.Background(), "fp-x")
	if err == nil {
		t.Fatal("GetShape expected deadline error")
	}
	_, err = st.RecordShapeHit(context.Background(), h)
	if err == nil {
		t.Fatal("RecordShapeHit expected deadline error")
	}
	err = st.ConfirmShape(context.Background(), "fp-x")
	if err == nil {
		t.Fatal("ConfirmShape expected deadline error")
	}
	err = st.RejectShape(context.Background(), "fp-x")
	if err == nil {
		t.Fatal("RejectShape expected deadline error")
	}
}

func TestConfirmShapeIncrements(t *testing.T) {
	st, ctx := testStore(t)
	h := testShapeHit("fp-f", 1)
	if _, err := st.RecordShapeHit(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err := st.ConfirmShape(ctx, "fp-f"); err != nil {
		t.Fatal(err)
	}
	row, _, err := st.GetShape(ctx, "fp-f")
	if err != nil || row.ConfirmCount != 1 {
		t.Fatalf("confirm_count=%d err=%v", row.ConfirmCount, err)
	}
}
