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

func TestListCompleteShapes(t *testing.T) {
	st, ctx := testStore(t)
	tok, _, err := st.Claim(ctx, "shape1")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "shape1", tok, StateRow{TermID: "t", Shape: "marketing-pitch promotional"}, nil); err != nil {
		t.Fatal(err)
	}
	shapes, err := st.ListCompleteShapes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shapes) != 1 || shapes[0] != "marketing-pitch promotional" {
		t.Fatalf("shapes %v", shapes)
	}
}

func TestListCompleteShapeRows_includesEmptyShape(t *testing.T) {
	st, ctx := testStore(t)
	tok, _, err := st.Claim(ctx, "empty-shape")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "empty-shape", tok, StateRow{TermID: "t", Shape: ""}, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListCompleteShapeRows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ObjectHash != "empty-shape" {
		t.Fatalf("rows %+v", rows)
	}
	shapes, err := st.ListCompleteShapes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shapes) != 0 {
		t.Fatalf("ListCompleteShapes should omit empty: %v", shapes)
	}
}

func TestRewriteShape(t *testing.T) {
	st, ctx := testStore(t)
	tok, _, err := st.Claim(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "h1", tok, StateRow{TermID: "t", Shape: "a with fyi"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.RewriteShape(ctx, "h1", "fyi"); err != nil {
		t.Fatal(err)
	}
	shapes, err := st.ListCompleteShapes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shapes) != 1 || shapes[0] != "fyi" {
		t.Fatalf("shapes %v", shapes)
	}
	if err := st.RewriteShape(ctx, "missing", "x"); err == nil {
		t.Fatal("expected missing hash error")
	}
	tok2, _, err := st.Claim(ctx, "running")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RewriteShape(ctx, "running", "x"); err == nil {
		t.Fatal("expected running row unchanged")
	}
	_ = st.Fail(ctx, "running", tok2, "err")
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
