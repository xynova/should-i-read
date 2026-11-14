package maildelete

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type holdDeleter struct {
	block chan struct{}
}

func (h *holdDeleter) MessageDelete(ctx context.Context, folder, envelopeID string) (*pimalaya.Result, error) {
	select {
	case <-h.block:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &pimalaya.Result{Raw: json.RawMessage(`{"action":"moved-to-trash","count":1}`)}, nil
}

func TestCheckoutClearsCartAndApplies(t *testing.T) {
	st, ctx := openTestStore(t)
	hold := &holdDeleter{block: make(chan struct{})}
	t.Cleanup(func() { close(hold.block) })
	if err := st.AddDeleteCart(ctx, []hoststore.DeleteCartItem{
		{ObjectHash: "keep-planned"},
		{ObjectHash: "already-trash"},
	}); err != nil {
		t.Fatal(err)
	}
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st,
		Pimdir: &fakePim{m: map[string]pimdir.EmailSummary{
			"keep-planned":  {Collection: "gmail/imap/[Gmail]/All Mail", Seq: 11},
			"already-trash": {Collection: "gmail/imap/[Gmail]/Trash", Seq: 22},
		}},
		Deleter:     hold,
		Now:         time.Now,
		ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	job, err := svc.Checkout(ctx, hoststore.DeleteJobConfirmToken)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" || job.Status != hoststore.DeleteJobStatusApplying {
		t.Fatalf("job %+v", job)
	}
	if job.Counts.Planned != 1 || job.Counts.Refused != 1 {
		t.Fatalf("counts %+v", job.Counts)
	}
	cart, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart) != 0 {
		t.Fatalf("cart after checkout %+v", cart)
	}
}

func TestCheckoutConfirmMismatch(t *testing.T) {
	st, ctx := openTestStore(t)
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st, Pimdir: &fakePim{}, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Checkout(ctx, "nope")
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatal(err)
	}
}

func TestCheckoutEmptyCartNoJob(t *testing.T) {
	st, ctx := openTestStore(t)
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st, Pimdir: &fakePim{}, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Checkout(ctx, hoststore.DeleteJobConfirmToken)
	if err == nil {
		t.Fatal("expected empty cart")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatal(err)
	}
}

func TestCheckoutEmptyCartRetriesFailedJob(t *testing.T) {
	st, ctx := openTestStore(t)
	hold := &holdDeleter{block: make(chan struct{})}
	t.Cleanup(func() { close(hold.block) })
	if err := st.AddDeleteCart(ctx, []hoststore.DeleteCartItem{{ObjectHash: "h1"}}); err != nil {
		t.Fatal(err)
	}
	jobID, err := st.CreateDeleteJobFromCart(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BindDeleteJobItems(ctx, jobID, []hoststore.DeleteJobBinding{
		{ObjectHash: "h1", Folder: "INBOX", EnvelopeID: "1", Status: hoststore.DeleteJobItemFailed},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDeleteJobStatus(ctx, jobID, hoststore.DeleteJobStatusPartial, "no id matched", nil); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveDeleteCart(ctx, "h1"); err != nil {
		t.Fatal(err)
	}
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st, Pimdir: &fakePim{}, Deleter: hold, Now: time.Now, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	job, err := svc.Checkout(ctx, hoststore.DeleteJobConfirmToken)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != jobID || job.Status != hoststore.DeleteJobStatusApplying {
		t.Fatalf("job %+v", job)
	}
	cart, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart) != 0 {
		t.Fatalf("cart %+v", cart)
	}
}

func TestCheckoutAllRefusedSkipsApply(t *testing.T) {
	st, ctx := openTestStore(t)
	if err := st.AddDeleteCart(ctx, []hoststore.DeleteCartItem{
		{ObjectHash: "already-trash"},
	}); err != nil {
		t.Fatal(err)
	}
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st,
		Pimdir: &fakePim{m: map[string]pimdir.EmailSummary{
			"already-trash": {Collection: "gmail/imap/[Gmail]/Trash", Seq: 22},
		}},
		Now:         time.Now,
		ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	job, err := svc.Checkout(ctx, hoststore.DeleteJobConfirmToken)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != hoststore.DeleteJobStatusPlanned {
		t.Fatalf("status %s", job.Status)
	}
	if job.Counts.Refused != 1 || job.Counts.Planned != 0 {
		t.Fatalf("counts %+v", job.Counts)
	}
	cart, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart) != 0 {
		t.Fatalf("cart %+v", cart)
	}
}
