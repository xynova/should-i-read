package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/pimalaya"
)

func TestFormatSyncReport_fixture(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	raw, err := os.ReadFile(filepath.Join("..", "pimalaya", "testdata", "sync-gmail-sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	rep := pimalaya.SummarizeNeverestJSON(raw)
	out := formatSyncReport(rep)
	for _, part := range []string{"Fetched 4", "removed from local store 2", "Gmail labels overlap"} {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q in:\n%s", part, out)
		}
	}
}
