package mailreport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateEssenceDumperEmptyDir(t *testing.T) {
	t.Parallel()
	if CreateEssenceDumper(EssenceDumpConfig{}) != nil {
		t.Fatal("expected nil dumper")
	}
}

func TestEssenceDumperWriteLast(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	d := CreateEssenceDumper(EssenceDumpConfig{Dir: dir, Now: now})
	path, err := d.writeLast("att3", "granite", "Subject: hi\n", "WHY1: a\nABOUT:\n", 3, errAboutOrShape())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got EssenceParseDump
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ObjectHash != "att3" || got.Attempt != 3 || got.Raw != "WHY1: a\nABOUT:\n" {
		t.Fatalf("got %+v", got)
	}
	if got.WrittenAt != "2026-10-13T12:00:00Z" {
		t.Fatalf("written_at %q", got.WrittenAt)
	}
	if !strings.Contains(got.ParseError, "about or shape empty") {
		t.Fatalf("parse_error %q", got.ParseError)
	}
}

func TestSanitizeEssenceDumpID(t *testing.T) {
	t.Parallel()
	if got := sanitizeEssenceDumpID("../etc/passwd"); strings.Contains(got, "/") {
		t.Fatalf("slash survived %q", got)
	}
	if got := sanitizeEssenceDumpID(""); got != essenceDumpIDUnknown {
		t.Fatalf("empty: %q", got)
	}
}

func TestWithEssenceDumpIDRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := withEssenceDumpID(context.Background(), "qmdiovp3")
	if essenceDumpIDFromCtx(ctx) != "qmdiovp3" {
		t.Fatalf("id=%q", essenceDumpIDFromCtx(ctx))
	}
}

func TestEssenceDumperOverwriteLastAttempt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d := CreateEssenceDumper(EssenceDumpConfig{Dir: dir, Now: time.Now().UTC()})
	if _, err := d.writeLast("h1", "m", "", "first", 1, errAboutOrShape()); err != nil {
		t.Fatal(err)
	}
	path, err := d.writeLast("h1", "m", "", "last", 3, errAboutOrShape())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "h1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"raw": "last"`) {
		t.Fatalf("want last attempt: %s", raw)
	}
	if path != filepath.Join(dir, "h1.json") {
		t.Fatalf("path %s", path)
	}
}

func errAboutOrShape() error {
	_, err := parseEssenceResponse("WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT:\nSHAPE:\nKIND: a > b > c")
	return err
}
