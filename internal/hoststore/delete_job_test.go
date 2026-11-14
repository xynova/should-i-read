package hoststore

import (
	"context"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func ctxWithDeadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestDeleteJobRoundTrip(t *testing.T) {
	st, ctx := testStore(t)
	now := time.Now().UTC()
	if err := st.AddDeleteCart(ctx, []DeleteCartItem{
		{ObjectHash: "hash-a", Sender: "a@x", Subject: "A"},
		{ObjectHash: "hash-b", Sender: "b@x", Subject: "B"},
	}); err != nil {
		t.Fatal(err)
	}
	jobID, err := st.CreateDeleteJobFromCart(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	job, err := st.GetDeleteJob(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != DeleteJobStatusPlanned || len(job.Items) != 2 {
		t.Fatalf("job: %+v", job)
	}
	if err := st.BindDeleteJobItems(ctx, jobID, []DeleteJobBinding{
		{
			ObjectHash: "hash-a",
			Folder:     "INBOX",
			EnvelopeID: "1",
			Seq:        1,
			Status:     DeleteJobItemPlanned,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDeleteJobItemStatus(ctx, jobID, "hash-b", DeleteJobItemTrashed, ""); err != nil {
		t.Fatal(err)
	}
	applyable, err := st.ListApplyableDeleteJobItems(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(applyable) != 1 || applyable[0].ObjectHash != "hash-a" {
		t.Fatalf("applyable: %+v", applyable)
	}
}

func TestDeleteJobEmptyCartInvalid(t *testing.T) {
	st, ctx := testStore(t)
	_, err := st.CreateDeleteJobFromCart(ctx, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func TestLatestDeleteJobItemsPrefersNewerJob(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.AddDeleteCart(ctx, []DeleteCartItem{{ObjectHash: "hash-a"}}); err != nil {
		t.Fatal(err)
	}
	oldID, err := st.CreateDeleteJobFromCart(ctx, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BindDeleteJobItems(ctx, oldID, []DeleteJobBinding{
		{ObjectHash: "hash-a", Folder: "INBOX", EnvelopeID: "1", Status: DeleteJobItemFailed},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddDeleteCart(ctx, []DeleteCartItem{{ObjectHash: "hash-a"}}); err != nil {
		t.Fatal(err)
	}
	newID, err := st.CreateDeleteJobFromCart(ctx, time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BindDeleteJobItems(ctx, newID, []DeleteJobBinding{
		{ObjectHash: "hash-a", Folder: "[Gmail]/Trash", EnvelopeID: "9", Status: DeleteJobItemRefused},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := st.LatestDeleteJobItems(ctx, []string{"hash-a", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if got["hash-a"].Status != DeleteJobItemRefused || got["hash-a"].JobID != newID {
		t.Fatalf("%+v", got["hash-a"])
	}
	if _, ok := got["missing"]; ok {
		t.Fatal("missing")
	}
}

func TestDeleteJobRequiresDeadline(t *testing.T) {
	st, _ := testStore(t)
	_, err := st.CreateDeleteJobFromCart(context.Background(), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := sirerr.AsCode(err); !ok {
		t.Fatal(err)
	}
}
