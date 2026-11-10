package pimalaya

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateHimalayaResolveFromCargoHome(t *testing.T) {
	dir := t.TempDir()
	cargoHome := filepath.Join(dir, "cargo")
	binDir := filepath.Join(cargoHome, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(binDir, "himalaya")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CARGO_HOME", cargoHome)
	t.Setenv("PATH", "/nonexistent")

	got, err := ResolveBin("", "himalaya")
	if err != nil {
		t.Fatal(err)
	}
	if got != script {
		t.Fatalf("got %q want %q", got, script)
	}
}
