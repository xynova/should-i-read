package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
)

func TestEssenceCacheAdapter_roundTrip(t *testing.T) {
	path := t.TempDir() + "/ledger.db"
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer st.Close()
	sys := DefaultEssenceSystemPrompt()
	cache, err := CreateEssenceCache(st, sys, "m", "")
	if err != nil {
		t.Fatal(err)
	}
	out := harness.EssenceOut{
		Why:   [5]string{"w", "w", "w", "w", "w"},
		About: "a",
		Shape: "fyi",
		Kind:  "notification > software-release > changelog",
	}
	if err := cache.Put(ctx, "h1", out); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	st2, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	cache2, err := CreateEssenceCache(st2, sys, "m", "")
	if err != nil {
		t.Fatal(err)
	}
	got, hit, err := cache2.Get(ctx, "h1")
	if err != nil || !hit {
		t.Fatalf("get hit=%v err=%v", hit, err)
	}
	if got.Kind != out.Kind {
		t.Fatalf("kind=%q", got.Kind)
	}
	wrong, err := CreateEssenceCache(st2, "other-prompt", "m", "")
	if err != nil {
		t.Fatal(err)
	}
	_, hit, err = wrong.Get(ctx, "h1")
	if err != nil || hit {
		t.Fatalf("prompt miss hit=%v err=%v", hit, err)
	}
}
