package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
)

func testEssenceStore(t *testing.T) (*hoststore.Store, context.Context) {
	path := t.TempDir() + "/ledger.db"
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = st.Close() })
	return st, ctx
}

func TestPrefetchEssence_skipsChatOnLedgerHit(t *testing.T) {
	st, ctx := testEssenceStore(t)
	sys := DefaultEssenceSystemPrompt()
	fp := essenceFingerprint("m", "", sys)
	parsed := harness.EssenceOut{
		Why:   [5]string{"w", "w", "w", "w", "w"},
		About: "a",
		Shape: "fyi",
		Kind:  "notification > software-release > changelog",
	}
	if err := st.PutEssence(ctx, "h1", fp, essenceRowFromOut(parsed)); err != nil {
		t.Fatal(err)
	}
	stub := &stubChatLines{}
	items := []Item{
		{ObjectHash: "h1", Body: "one"},
		{ObjectHash: "h2", Body: "two"},
	}
	got, errs, err := prefetchEssence(ctx, st, stub, "m", "", WorldContextAttach, sys, items, 8)
	if err != nil || len(errs) != 0 {
		t.Fatalf("prefetch err=%v errs=%v", err, errs)
	}
	if stub.calls.Load() != 1 {
		t.Fatalf("chat batches=%d want 1 (h1 cached)", stub.calls.Load())
	}
	if len(stub.last) != 1 || stub.last[0].CustomID != "h2" {
		t.Fatalf("unexpected chat lines: %+v", stub.last)
	}
	if got["h1"].Kind != parsed.Kind {
		t.Fatalf("h1 kind=%q", got["h1"].Kind)
	}
}

func TestPrefetchEssence_putErrorFailsRun(t *testing.T) {
	st, ctx := testEssenceStore(t)
	_ = st.Close()
	stub := &stubChatLines{}
	items := []Item{{ObjectHash: "h1", Body: "one"}}
	_, _, err := prefetchEssence(ctx, st, stub, "m", "", WorldContextAttach, DefaultEssenceSystemPrompt(), items, 8)
	if err == nil {
		t.Fatal("expected put error from closed store")
	}
}
