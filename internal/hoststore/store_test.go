package hoststore

import (
	"context"
	"testing"
	"time"
)

func testStore(t *testing.T) (*Store, context.Context) {
	path := t.TempDir() + "/ledger.db"
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	st, err := CreateStore(StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = st.Close() })
	return st, ctx
}

func TestCreateStoreTwiceMigratesOnce(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ledger.db"
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s1, err := CreateStore(StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	var version int64
	var dirty bool
	if err := s1.db.QueryRow(`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if version != 4 {
		t.Fatalf("schema_migrations version=%d, want 4", version)
	}
	if dirty {
		t.Fatal("schema_migrations dirty")
	}
	_ = s1.Close()
	s2, err := CreateStore(StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	_ = s2.Close()
}

func TestCreateStoreRequiresNow(t *testing.T) {
	_, err := CreateStore(StoreConfig{Path: t.TempDir() + "/x.db", Now: time.Time{}})
	if err == nil {
		t.Fatal("expected error for zero Now")
	}
}

func TestStoreMethodsRequireDeadline(t *testing.T) {
	st, _ := testStore(t)
	_, _, err := st.Claim(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}
