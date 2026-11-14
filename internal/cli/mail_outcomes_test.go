package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
)

func TestMailOutcomesExportHelp(t *testing.T) {
	cmd := newMailOutcomesExportCmd(&rootOptions{}, t.TempDir())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Help(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("export")) {
		t.Fatalf("help: %s", buf.String())
	}
}

func TestMailOutcomesExportWritesCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	if err := writeOutcomesCSV(path, []hoststore.MessageOutcome{
		{
			ObjectHash: "h1",
			Outcome:    hoststore.OutcomeKept,
			Source:     hoststore.OutcomeSourceOverviewKeep,
			Sender:     "a@x",
			DecidedAt:  now.UTC().Format(time.RFC3339),
		},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "object_hash,outcome,source,job_id,sender,decided_at"
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	if len(lines) < 1 || string(lines[0]) != want {
		t.Fatalf("header: %q", lines[0])
	}
}
