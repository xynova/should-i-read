package mailreport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
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

func TestLoadCatalog_repairsAliasCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.yaml")
	body := `vocabulary:
  id: demo
  terms:
    - id: n
      label: N
      description: d
    - id: ci
      label: CI
      parent: n
      description: d
    - id: k
      label: K
      parent: ci
      description: d
      aliases:
        - n > ci > k
    - id: loser
      label: L
      parent: ci
      description: d
      aliases:
        - n > ci > k
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, vocab, err := LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.BuildCatalog(vocab); err != nil {
		t.Fatal(err)
	}
	var winnerAlias, loserAlias int
	for _, term := range vocab.Terms {
		switch term.ID {
		case "k":
			winnerAlias = len(term.Aliases)
		case "loser":
			loserAlias = len(term.Aliases)
		}
	}
	if winnerAlias != 1 || loserAlias != 0 {
		t.Fatalf("winner aliases=%d loser aliases=%d", winnerAlias, loserAlias)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "n > ci > k") {
		t.Fatal("repaired catalog should still contain winner alias")
	}
}

func TestLoadCatalog_cycleStillFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.yaml")
	body := `vocabulary:
  id: demo
  terms:
    - id: a
      label: A
      parent: b
      description: d
    - id: b
      label: B
      parent: a
      description: d
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = LoadCatalog(path)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("cycle catalog must not be rewritten")
	}
}

func TestPersistCatalogIfBuildable_doesNotSaveWhenBuildFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.yaml")
	body := `vocabulary:
  id: demo
  terms:
    - id: a
      label: A
      description: d
      aliases:
        - shared
    - id: b
      label: B
      description: d
      aliases:
        - shared
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	vocab, err := catalog.ParseYAML(before)
	if err != nil {
		t.Fatal(err)
	}
	_, err = persistCatalogIfBuildable(path, vocab)
	if err == nil {
		t.Fatal("expected build failure")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("must not save when build fails")
	}
}
