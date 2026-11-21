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

func TestLookupActionedMissingReplicaIsAlreadyGone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pim := &fakePim{m: map[string]pimdir.EmailSummary{
		"live": {Collection: "INBOX", Sender: "a@x", Seq: 1},
	}}
	jobs := map[string]hoststore.DeleteJobItem{
		"ghost": {Status: hoststore.DeleteJobItemUnresolved, Sender: "(unknown sender)"},
	}
	got, err := LookupActioned(ctx, pim, jobs, []string{"live", "ghost"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["live"]; ok {
		t.Fatal("live replica must stay on the board")
	}
	if got["ghost"].Outcome != OutcomeAlreadyInTrash {
		t.Fatalf("missing replica %+v", got["ghost"])
	}
}

func TestLookupActionedNilPimDoesNotHideMissing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := LookupActioned(ctx, nil, map[string]hoststore.DeleteJobItem{
		"ghost": {Status: hoststore.DeleteJobItemUnresolved},
	}, []string{"ghost"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["ghost"]; ok {
		t.Fatalf("nil pimdir must not hide unresolved: %+v", got["ghost"])
	}
}

func TestMergeTrashOutcomesHidesLedgerTrash(t *testing.T) {
	infos := map[string]ActionedInfo{
		"moved": {ObjectHash: "moved", Outcome: OutcomeTrashedAtSource, Sender: "job@x"},
	}
	outcomes := map[string]hoststore.MessageOutcome{
		"moved":  {ObjectHash: "moved", Outcome: hoststore.OutcomeAlreadyInTrash, Sender: "ledger@x"},
		"gone":   {ObjectHash: "gone", Outcome: hoststore.OutcomeTrashedAtSource, Sender: "n@github.com"},
		"kept":   {ObjectHash: "kept", Outcome: hoststore.OutcomeKept, Sender: "keep@x"},
		"failed": {ObjectHash: "failed", Outcome: hoststore.OutcomeAlreadyInTrash, Sender: "old@x"},
	}
	got := MergeTrashOutcomes(infos, outcomes)
	if got["moved"].Outcome != OutcomeTrashedAtSource || got["moved"].Sender != "job@x" {
		t.Fatalf("existing classify must win: %+v", got["moved"])
	}
	if got["gone"].Outcome != OutcomeTrashedAtSource || got["gone"].Sender != "n@github.com" {
		t.Fatalf("ledger trash must hide: %+v", got["gone"])
	}
	if got["failed"].Outcome != OutcomeAlreadyInTrash {
		t.Fatalf("already in Trash must hide: %+v", got["failed"])
	}
	if _, ok := got["kept"]; ok {
		t.Fatal("kept must stay live")
	}
}

func TestMergeTrashOutcomesHidesAcked(t *testing.T) {
	got := MergeTrashOutcomes(nil, map[string]hoststore.MessageOutcome{
		"ack": {ObjectHash: "ack", Outcome: hoststore.OutcomeAcked, Sender: "a@x"},
	})
	if got["ack"].Outcome != OutcomeAckedAtSource {
		t.Fatalf("acked must hide: %+v", got["ack"])
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

func TestPersistActionedMissingReplica(t *testing.T) {
	st, ctx := testPersistStore(t)
	ctxLookup, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	jobs := map[string]hoststore.DeleteJobItem{
		"ghost": {Status: hoststore.DeleteJobItemUnresolved, Sender: "(unknown sender)"},
	}
	infos, err := LookupActioned(ctxLookup, &fakePim{m: map[string]pimdir.EmailSummary{}}, jobs, []string{"ghost"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PersistActioned(ctx, st, jobs, infos); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetOutcome(ctx, "ghost")
	if err != nil || !ok || got.Outcome != hoststore.OutcomeAlreadyInTrash || got.Source != hoststore.OutcomeSourceReplicaFolder {
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
