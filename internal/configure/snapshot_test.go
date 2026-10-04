package configure

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xynova/should-i-read/internal/config"
)

func TestCollectSnapshotNoHostConfig(t *testing.T) {
	dir := t.TempDir()
	xdg := filepath.Join(dir, "xdg")
	appCfgDir := filepath.Join(xdg, config.AppName)
	if err := os.MkdirAll(appCfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)

	ctx := context.Background()
	snap, err := CollectSnapshot(ctx, t.TempDir(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Steps[0].ID != StepHostConfig || snap.Steps[0].OK {
		t.Fatalf("host step: %+v", snap.Steps[0])
	}
	if snap.Ready {
		t.Fatal("expected not ready")
	}
}

func TestStepOrderLength(t *testing.T) {
	if len(StepOrder) != 6 {
		t.Fatalf("got %d", len(StepOrder))
	}
}
