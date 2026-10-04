package pimalaya

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
	res, err := n.Sync(context.Background(), "gmail")
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
