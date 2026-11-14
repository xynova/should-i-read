package maildelete

import (
	"context"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimdir"
)

type fakePim struct {
	m map[string]pimdir.EmailSummary
}

func (f *fakePim) SummariesByObjectHash(ctx context.Context, hashes []string) (map[string]pimdir.EmailSummary, error) {
	out := map[string]pimdir.EmailSummary{}
	for _, h := range hashes {
		if s, ok := f.m[h]; ok {
			out[h] = s
		}
	}
	return out, nil
}

func TestBindingFromSummary(t *testing.T) {
	b := BindingFromSummary("abcd1234efgh5678", pimdir.EmailSummary{
		Collection: "INBOX",
		Seq:        1,
	}, true)
	if b.Status != hoststore.DeleteJobItemPlanned || b.Folder != "INBOX" || b.EnvelopeID != "1" {
		t.Fatalf("%+v", b)
	}
	b = BindingFromSummary("x", pimdir.EmailSummary{}, false)
	if b.Status != hoststore.DeleteJobItemUnresolved {
		t.Fatalf("%+v", b)
	}
	b = BindingFromSummary("y", pimdir.EmailSummary{Collection: "[Gmail]/Trash", Seq: 1}, true)
	if b.Status != hoststore.DeleteJobItemRefused {
		t.Fatalf("%+v", b)
	}
}

func TestResolveJobBindings(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pim := &fakePim{m: map[string]pimdir.EmailSummary{
		"h1": {Collection: "imap/INBOX", Seq: 42},
	}}
	out, err := ResolveJobBindings(ctx, pim, []string{"h1", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatal(out)
	}
	if out[0].EnvelopeID != "42" || out[0].Folder != "imap/INBOX" {
		t.Fatalf("%+v", out[0])
	}
	if out[1].Status != hoststore.DeleteJobItemUnresolved {
		t.Fatalf("%+v", out[1])
	}
}

func TestBindingFromSummaryImapCollection(t *testing.T) {
	b := BindingFromSummary("h", pimdir.EmailSummary{
		Collection: "imap/INBOX",
		Seq:        42,
	}, true)
	if b.Folder != "imap/INBOX" || b.EnvelopeID != "42" {
		t.Fatalf("%+v", b)
	}
}

func TestStartJobDropsAlreadyTrashCart(t *testing.T) {
	st, ctx := openTestStore(t)
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
		Now:         time.Now,
		ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	job, err := svc.StartJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if job.Counts.Planned != 1 || job.Counts.Refused != 1 {
		t.Fatalf("counts %+v", job.Counts)
	}
	cart, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart) != 1 || cart[0].ObjectHash != "keep-planned" {
		t.Fatalf("cart %+v", cart)
	}
}
