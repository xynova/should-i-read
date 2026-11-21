package pimalaya

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
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

func TestCreateHimalayaMissingConfig(t *testing.T) {
	dir := t.TempDir()
	cargoHome := filepath.Join(dir, "cargo")
	binDir := filepath.Join(cargoHome, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(binDir, "himalaya")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CARGO_HOME", cargoHome)
	t.Setenv("PATH", "/nonexistent")

	_, err := CreateHimalaya(config.Config{})
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeUnavailable {
		t.Fatalf("code %v err %v", code, err)
	}
}

func TestCreateHimalayaMissingFile(t *testing.T) {
	dir := t.TempDir()
	cargoHome := filepath.Join(dir, "cargo")
	binDir := filepath.Join(cargoHome, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(binDir, "himalaya")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CARGO_HOME", cargoHome)
	t.Setenv("PATH", "/nonexistent")

	missing := filepath.Join(dir, "missing.toml")
	_, err := CreateHimalaya(config.Config{
		Pimalaya: config.PimalayaConfig{HimalayaConfig: missing},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeUnavailable {
		t.Fatalf("code %v err %v", code, err)
	}
}

func TestFlagAddRejectsNonSeen(t *testing.T) {
	h := &Himalaya{Runner: &Runner{Bin: "himalaya"}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := h.FlagAdd(ctx, "INBOX", "1", "junk")
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code %v err %v", code, err)
	}
}
