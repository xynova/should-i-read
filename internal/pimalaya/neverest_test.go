package pimalaya

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func neverestCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestNeverestSyncUsesAccountFlagNotPositional(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "neverest")
	body := `#!/bin/sh
printf '%s\n' "$@" >&2
echo '{"ok":true}'
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	runner, err := Create(script)
	if err != nil {
		t.Fatal(err)
	}
	n := &Neverest{Runner: runner}
	res, err := n.Sync(neverestCtx(t), "gmail", nil)
	if err != nil {
		t.Fatal(err)
	}
	argsLine := strings.TrimSpace(res.Stderr)
	if argsLine == "" {
		t.Fatal("expected captured argv on stderr")
	}
	if strings.Contains(argsLine, " sync gmail") || strings.HasSuffix(argsLine, " gmail") {
		t.Fatalf("account must not be positional: %q", argsLine)
	}
	if !strings.Contains(argsLine, "-a") || !strings.Contains(argsLine, "gmail") {
		t.Fatalf("expected -a gmail in argv: %q", argsLine)
	}
	if !strings.Contains(argsLine, "sync") {
		t.Fatalf("expected sync subcommand: %q", argsLine)
	}
}

func TestNeverestSyncPassesExcludeCollections(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "neverest")
	body := `#!/bin/sh
printf '%s\n' "$@" >&2
echo '{"ok":true}'
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	runner, err := Create(script)
	if err != nil {
		t.Fatal(err)
	}
	n := &Neverest{Runner: runner}
	res, err := n.Sync(neverestCtx(t), "gmail", []string{"[Gmail]/Spam", "", "[Gmail]/Trash"})
	if err != nil {
		t.Fatal(err)
	}
	argsLine := strings.TrimSpace(res.Stderr)
	if !strings.Contains(argsLine, "--exclude-collection") || !strings.Contains(argsLine, "[Gmail]/Spam") {
		t.Fatalf("expected exclude flags in argv: %q", argsLine)
	}
	if !strings.Contains(argsLine, "[Gmail]/Trash") {
		t.Fatalf("expected trash exclude in argv: %q", argsLine)
	}
}

func TestNeverestSyncAcceptingAbsences(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "neverest")
	body := `#!/bin/sh
printf '%s\n' "$@" >&2
echo '{"ok":true}'
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	runner, err := Create(script)
	if err != nil {
		t.Fatal(err)
	}
	n := &Neverest{Runner: runner}
	res, err := n.SyncAcceptingAbsences(neverestCtx(t), "gmail", []string{"[Gmail]/Spam"})
	if err != nil {
		t.Fatal(err)
	}
	argsLine := strings.TrimSpace(res.Stderr)
	if !strings.Contains(argsLine, "--accept-absences") {
		t.Fatalf("expected --accept-absences in argv: %q", argsLine)
	}
	plain, err := n.Sync(neverestCtx(t), "gmail", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.TrimSpace(plain.Stderr), "--accept-absences") {
		t.Fatalf("plain sync must not pass --accept-absences: %q", plain.Stderr)
	}
}
