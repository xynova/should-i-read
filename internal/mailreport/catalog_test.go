package mailreport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCatalog_copiesSeedOnce(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.yaml")
	catalog := filepath.Join(dir, "runtime", "inbox-mail.yaml")
	seedBody := []byte("vocabulary:\n  id: inbox-mail\n  terms: []\n")
	if err := os.WriteFile(seed, seedBody, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCatalog(catalog, seed); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(seedBody) {
		t.Fatalf("unexpected catalog body: %q", got)
	}
	dirty := []byte("vocabulary:\n  id: inbox-mail\n  terms:\n    - id: dirty\n")
	if err := os.WriteFile(catalog, dirty, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCatalog(catalog, seed); err != nil {
		t.Fatal(err)
	}
	got2, err := os.ReadFile(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if string(got2) != string(dirty) {
		t.Fatal("EnsureCatalog must not overwrite existing catalog")
	}
}
