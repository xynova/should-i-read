package pimalaya

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFetchedHunks_fixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "sync-gmail-sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	hunks := FetchedHunks(raw)
	if len(hunks) != 4 {
		t.Fatalf("expected 4 fetch hunks, got %d", len(hunks))
	}
}
