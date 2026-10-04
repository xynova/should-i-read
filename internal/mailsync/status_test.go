package mailsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/config"
)

func TestCollectStatusNoConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := CollectStatus(ctx, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if st.ConfigOK {
		t.Fatal("expected config_ok false")
	}
	if st.Ready {
		t.Fatal("expected not ready")
	}
}

func TestCollectStatusConfigTokenNoDB(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mailPath := filepath.Join(dir, "mail-sync.toml")
	body := `[accounts.gmail]
imap.sasl.xoauth2.username = "me@example.com"
`
	if err := os.WriteFile(mailPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(dir, "store")
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{}
	cfg.Pimalaya.NeverestConfig = mailPath
	cfg.Pimalaya.PimdirPath = store
	cfg.Pimalaya.DefaultAccount = "gmail"

	st, err := CollectStatus(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ConfigOK {
		t.Fatal("expected config_ok")
	}
	if st.StoreOK {
		t.Fatal("expected store_ok false without pimdir.db")
	}
	if st.Ready {
		t.Fatal("expected not ready")
	}
}

func TestCollectStatusAllFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mailPath := filepath.Join(dir, "mail-sync.toml")
	if err := os.WriteFile(mailPath, []byte(`[accounts.gmail]`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(dir, "store")
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(store, "pimdir.db")
	if err := os.WriteFile(dbPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{}
	cfg.Pimalaya.NeverestConfig = mailPath
	cfg.Pimalaya.PimdirPath = store
	cfg.Pimalaya.DefaultAccount = "gmail"

	st, err := CollectStatus(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ConfigOK || !st.StoreOK {
		t.Fatalf("config=%v store=%v", st.ConfigOK, st.StoreOK)
	}
	// dep_ok and token_ok may be false in CI; ready requires all four
	if st.Ready && (!st.DepOK || !st.TokenOK) {
		t.Fatal("ready inconsistent")
	}
}

func TestProductNextStepsNoPim(t *testing.T) {
	t.Parallel()
	for _, step := range productNextSteps(Status{ConfigOK: false}) {
		if containsSubsystemVerb(step) {
			t.Fatalf("product step mentions subsystem: %q", step)
		}
	}
}

func containsSubsystemVerb(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "neverest") ||
		strings.Contains(lower, "pim ensure") ||
		strings.Contains(lower, "pim configure") ||
		strings.Contains(lower, "mail setup")
}
