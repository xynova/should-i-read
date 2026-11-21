package pimalaya

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func runJSONCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestResolveBinCargoHomeFallback(t *testing.T) {
	dir := t.TempDir()
	cargoHome := filepath.Join(dir, "cargo")
	binDir := filepath.Join(cargoHome, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(binDir, "neverest")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CARGO_HOME", cargoHome)
	t.Setenv("PATH", "/nonexistent")

	got, err := ResolveBin("", "neverest")
	if err != nil {
		t.Fatal(err)
	}
	if got != script {
		t.Fatalf("got %q want %q", got, script)
	}
}

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

func TestRunJSONRejectsMissingDeadline(t *testing.T) {
	t.Parallel()
	r, err := Create("neverest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.RunJSON(context.Background(), "check"); err == nil {
		t.Fatal("expected missing deadline")
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
	_, runErr := r.RunJSON(runJSONCtx(t), "check")
	if runErr == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(runErr)
	if !ok || code != sirerr.CodeNeedsReview {
		t.Fatalf("code: %v %q", ok, code)
	}
}

func TestCliDetailPrefersJSONStdout(t *testing.T) {
	t.Parallel()
	jsonOut := `{"ok":false,"error":{"code":"conflict"}}`
	stderr := "\x1b[m✓\x1b[m Resolved credentials"
	got := cliDetail(stderr, jsonOut)
	if !strings.Contains(got, "conflict") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "\x1b") || strings.Contains(got, "Resolved credentials") {
		t.Fatalf("got %q", got)
	}
	got = cliDetail("", jsonOut)
	if got != jsonOut {
		t.Fatalf("got %q", got)
	}
	got = cliDetail("boom", "plain text not json")
	if got != "boom" {
		t.Fatalf("got %q", got)
	}
	got = cliDetail("{\"event\":\"account-started\"}\n", "")
	if got != "" {
		t.Fatalf("got %q want empty when only progress events on stderr", got)
	}
}

func TestRunJSONPrefersStdoutWhenStderrHasProgress(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "neverest")
	body := `#!/bin/sh
printf '\033[m✓\033[m Resolved credentials\n' >&2
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
	_, runErr := r.RunJSON(runJSONCtx(t), "sync")
	if runErr == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(runErr)
	if !ok || code != sirerr.CodeNeedsReview {
		t.Fatalf("code: %v %q", ok, code)
	}
	msg := runErr.Error()
	if !strings.Contains(msg, "conflict") {
		t.Fatalf("want conflict in %q", msg)
	}
	if strings.Contains(msg, "Resolved credentials") || strings.Contains(msg, "\x1b") {
		t.Fatalf("want no progress stderr in %q", msg)
	}
}

func TestRunJSONUsesStdoutWhenStderrEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "himalaya")
	body := `#!/bin/sh
echo '{"error":"Cannot determine the trash mailbox; set ` + "`mailbox.alias.trash`" + ` in your config"}'
exit 1
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := Create(script)
	if err != nil {
		t.Fatal(err)
	}
	_, runErr := r.RunJSON(runJSONCtx(t), "message", "delete")
	if runErr == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(runErr.Error(), "Cannot determine the trash mailbox") {
		t.Fatalf("want stdout JSON in error, got %q", runErr.Error())
	}
}

func TestRunJSONStreamLineHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "neverest")
	body := `#!/bin/sh
echo '{"event":"scan-progressed","source":"imap","done":1,"total":9}'
echo '{"event":"scan-progressed","source":"imap","done":2,"total":9}' >&2
echo '{"ok":true}'
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := Create(script)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var got []string
	r.OnStreamLine = func(line []byte) {
		mu.Lock()
		got = append(got, string(line))
		mu.Unlock()
	}
	res, runErr := r.RunJSON(runJSONCtx(t), "sync")
	if runErr != nil {
		t.Fatal(runErr)
	}
	if res == nil || res.Exit != 0 {
		t.Fatalf("res %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, `"done":1`) || !strings.Contains(joined, `"done":2`) {
		t.Fatalf("got %v", got)
	}
}
