package pimalaya

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestAssertAllowed(t *testing.T) {
	t.Parallel()
	if err := AssertAllowed("neverest"); err != nil {
		t.Fatal(err)
	}
	if err := AssertAllowed("/usr/local/bin/himalaya"); err != nil {
		t.Fatal(err)
	}
	if err := AssertAllowed("curl"); err == nil {
		t.Fatal("expected reject")
	}
}

func TestRunJSONNeedsReviewExit2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "neverest")
	body := `#!/bin/sh
echo '{"ok":false,"error":{"code":"conflict"}}'
exit 2
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := Create(script)
	if err != nil {
		t.Fatal(err)
	}
	_, runErr := r.RunJSON(context.Background(), "check")
	if runErr == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(runErr)
	if !ok || code != sirerr.CodeNeedsReview {
		t.Fatalf("code: %v %q", ok, code)
	}
}
