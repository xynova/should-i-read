package maildelete

import (
	"context"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimdir"
)

func TestClassifyActioned(t *testing.T) {
	if ClassifyActioned(hoststore.DeleteJobItemFailed, "gmail/imap/[Gmail]/All Mail") != "" {
		t.Fatal("failed All Mail must stay live")
	}
	if ClassifyActioned(hoststore.DeleteJobItemPlanned, "gmail/imap/[Gmail]/All Mail") != "" {
		t.Fatal("planned All Mail must stay live")
	}
	if ClassifyActioned("", "gmail/imap/[Gmail]/Trash") != OutcomeAlreadyInTrash {
		t.Fatal("replica Trash is already in Trash")
	}
	if ClassifyActioned(hoststore.DeleteJobItemTrashed, "gmail/imap/[Gmail]/All Mail") != OutcomeTrashedAtSource {
		t.Fatal("trashed job item is trashed at source")
	}
	if ClassifyActioned(hoststore.DeleteJobItemRefused, "") != OutcomeAlreadyInTrash {
		t.Fatal("refused without folder is already in Trash")
	}
}

func TestLookupActionedTrashAndFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pim := &fakePim{m: map[string]pimdir.EmailSummary{
		"live":   {Collection: "gmail/imap/[Gmail]/All Mail", Sender: "a@x", Date: "2026-11-01"},
		"trash":  {Collection: "gmail/imap/[Gmail]/Trash", Sender: "notifications@github.com", Date: "2026-09-19"},
		"failed": {Collection: "gmail/imap/[Gmail]/All Mail", Sender: "notifications@github.com", Date: "2026-11-02"},
	}}
	jobs := map[string]hoststore.DeleteJobItem{
		"failed": {Status: hoststore.DeleteJobItemFailed, Folder: "gmail/imap/[Gmail]/All Mail"},
		"moved":  {Status: hoststore.DeleteJobItemTrashed, Sender: "b@x"},
	}
	got, err := LookupActioned(ctx, pim, jobs, []string{"live", "trash", "failed", "moved"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["live"]; ok {
		t.Fatal("live")
	}
	if _, ok := got["failed"]; ok {
		t.Fatal("failed")
	}
	if got["trash"].Outcome != OutcomeAlreadyInTrash {
		t.Fatalf("trash %+v", got["trash"])
	}
	if got["moved"].Outcome != OutcomeTrashedAtSource {
		t.Fatalf("moved %+v", got["moved"])
	}
}

func TestPersistActionedSkipsFailed(t *testing.T) {
	st, ctx := testPersistStore(t)
	ctxLookup, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	jobs := map[string]hoststore.DeleteJobItem{
		"failed": {Status: hoststore.DeleteJobItemFailed, Folder: "gmail/imap/[Gmail]/All Mail"},
	}
	infos, err := LookupActioned(ctxLookup, nil, jobs, []string{"failed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Fatalf("lookup must omit failed: %+v", infos)
	}
	if err := PersistActioned(ctx, st, jobs, infos); err != nil {
		t.Fatal(err)
	}
	_, ok, err := st.GetOutcome(ctx, "failed")
	if err != nil || ok {
		t.Fatalf("failed hash must not persist: ok %v err %v", ok, err)
	}
}

func TestPersistActionedReplicaFolder(t *testing.T) {
	st, ctx := testPersistStore(t)
	infos := map[string]ActionedInfo{
		"trash": {
			ObjectHash: "trash",
			Outcome:    OutcomeAlreadyInTrash,
			Sender:     "n@github.com",
			Folder:     "gmail/imap/[Gmail]/Trash",
		},
	}
	if err := PersistActioned(ctx, st, nil, infos); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetOutcome(ctx, "trash")
	if err != nil || !ok || got.Source != hoststore.OutcomeSourceReplicaFolder {
		t.Fatalf("got %+v ok %v err %v", got, ok, err)
	}
}

func testPersistStore(t *testing.T) (*hoststore.Store, context.Context) {
	t.Helper()
	path := t.TempDir() + "/ledger.db"
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = st.Close() })
	return st, ctx
}
