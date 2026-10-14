package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailsync"
)

func TestCollectClassifyStoreStatus(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "should-i-read.db")
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: db, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tok, _, err := st.Claim(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Complete(ctx, "h1", tok, hoststore.StateRow{TermID: "t"}, nil); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	cfg := config.Config{StorePath: db}
	got := collectClassifyStoreStatus(ctx, cfg)
	if got.Path != db || got.Complete != 1 {
		t.Fatalf("%+v", got)
	}
	raw, err := json.Marshal(mailsync.Status{ClassifyStore: got})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"classify_store"`) || !strings.Contains(s, `"complete"`) {
		t.Fatalf("json: %s", raw)
	}
}
