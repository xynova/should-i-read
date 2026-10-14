package hoststore

import (
	"testing"
	"time"
)

func TestClaimCompleteGet(t *testing.T) {
	st, ctx := testStore(t)
	tok, skip, err := st.Claim(ctx, "hash1")
	if err != nil || skip || tok == "" {
		t.Fatalf("Claim: skip=%v tok=%q err=%v", skip, tok, err)
	}
	row := StateRow{
		TermID: "changelog",
		Label:  "changelog",
		Source: "breadcrumb",
	}
	if err := st.Complete(ctx, "hash1", tok, row, nil); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(ctx, "hash1")
	if err != nil {
		t.Fatal(err)
	}
	if got.TermID != "changelog" || got.Status != "complete" {
		t.Fatalf("got %+v", got)
	}
	_, skip2, err := st.Claim(ctx, "hash1")
	if err != nil || !skip2 {
		t.Fatalf("second claim: skip=%v err=%v", skip2, err)
	}
}

func TestFailThenRetryClaim(t *testing.T) {
	st, ctx := testStore(t)
	tok, _, err := st.Claim(ctx, "hash2")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Fail(ctx, "hash2", tok, "essence failed"); err != nil {
		t.Fatal(err)
	}
	tok2, skip, err := st.Claim(ctx, "hash2")
	if err != nil || skip || tok2 == "" {
		t.Fatalf("retry claim: skip=%v err=%v", skip, err)
	}
}

func TestStaleCompleteToken(t *testing.T) {
	st, ctx := testStore(t)
	tok, _, err := st.Claim(ctx, "hash3")
	if err != nil {
		t.Fatal(err)
	}
	err = st.Complete(ctx, "hash3", tok, StateRow{TermID: "x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = st.Complete(ctx, "hash3", tok, StateRow{TermID: "y"}, nil)
	if err == nil {
		t.Fatal("expected stale token error")
	}
}

func TestImportProgressOnce(t *testing.T) {
	st, ctx := testStore(t)
	n, err := st.ImportProgress(ctx, map[string]ProgressEntry{
		"h1": {TermID: "t1", UpdatedAt: time.RFC3339},
	})
	if err != nil || n != 1 {
		t.Fatalf("import n=%d err=%v", n, err)
	}
	n2, err := st.ImportProgress(ctx, map[string]ProgressEntry{"h2": {TermID: "t2"}})
	if err != nil || n2 != 0 {
		t.Fatalf("second import n=%d err=%v", n2, err)
	}
	c, err := st.Counts(ctx)
	if err != nil || c.Complete != 1 {
		t.Fatalf("counts %+v err=%v", c, err)
	}
}
