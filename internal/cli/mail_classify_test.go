package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestIsClassifyUnavailable(t *testing.T) {
	t.Parallel()
	if isClassifyUnavailable(sirerr.New(sirerr.CodeUnavailable, "op", "polypus down")) {
		return
	}
	t.Fatal("expected unavailable")
}

func TestIsClassifyUnavailableFalse(t *testing.T) {
	t.Parallel()
	if isClassifyUnavailable(sirerr.New(sirerr.CodeFailed, "op", "other")) {
		t.Fatal("failed should not skip")
	}
	if isClassifyUnavailable(nil) {
		t.Fatal("nil should not skip")
	}
}

func TestMailReportWindow(t *testing.T) {
	t.Parallel()
	w, err := validateMailReportWindow("", "2026-09-01", "2026-09-08")
	if err != nil || w.Zero() {
		t.Fatalf("valid window: %v %v", w, err)
	}
	_, err = validateMailReportWindow("", "2026-09-01", "")
	if err == nil {
		t.Fatal("expected missing --to")
	}
	_, err = validateMailReportWindow("", "", "2026-09-08")
	if err == nil {
		t.Fatal("expected missing --from")
	}
	_, err = validateMailReportWindow("export.json", "2026-09-01", "2026-09-08")
	if err == nil {
		t.Fatal("expected --in conflict")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func TestReportListLimit(t *testing.T) {
	t.Parallel()
	window, err := pimdir.CreateDateWindow("2026-09-01", "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if reportListLimit(25, false, window) != 0 {
		t.Fatal("window without changed limit should be uncapped")
	}
	if reportListLimit(3, true, window) != 3 {
		t.Fatal("window with changed limit")
	}
	if reportListLimit(25, false, pimdir.DateWindow{}) != 25 {
		t.Fatal("no window uses limit")
	}
}

func TestClassifyJobContextNil(t *testing.T) {
	t.Parallel()
	var parent context.Context
	_, _, err := classifyJobContext(parent, false) //nolint:staticcheck // tests the nil-context fail-closed branch
	if err == nil {
		t.Fatal("expected nil context error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func TestClassifyJobContextDeadline(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx, stop, err := classifyJobContext(parent, false)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected deadline")
	}
}

func TestTruncateRunesUTF8(t *testing.T) {
	t.Parallel()
	got := truncateRunes("café-menu-item", 6)
	if got != "caf..." {
		t.Fatalf("got %q", got)
	}
	if truncateRunes("ab", 0) != "" {
		t.Fatal("max 0")
	}
	if truncateRunes("abcd", 3) != "abc" {
		t.Fatalf("max 3: %q", truncateRunes("abcd", 3))
	}
}

func TestReportFileStamp(t *testing.T) {
	t.Parallel()
	got, err := reportFileStamp("2026-10-17T01:02:03Z")
	if err != nil {
		t.Fatal(err)
	}
	if got != "20261017T010203Z" {
		t.Fatalf("stamp %q", got)
	}
	_, err = reportFileStamp("")
	if err == nil {
		t.Fatal("expected empty generated_at")
	}
}

func TestWriteReportArtifactUsesGeneratedAt(t *testing.T) {
	dir := t.TempDir()
	path, err := writeReportArtifact(dir, "", mailreport.Artifact{GeneratedAt: "2026-10-17T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "tmp", "unwanted-report-20261017T010203Z.json")
	if path != want {
		t.Fatalf("path %q want %q", path, want)
	}
}

func TestValidateMailReportWindowWrapsPimdir(t *testing.T) {
	t.Parallel()
	_, err := validateMailReportWindow("", "2026-09-01", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cli.validateMailReportWindow") {
		t.Fatalf("missing CLI op: %v", err)
	}
}
