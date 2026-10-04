package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultMailSyncConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got, err := DefaultMailSyncConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, AppName, mailSyncConfigName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultPimStoreDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got := DefaultPimStoreDir("work/inbox")
	if got == "" {
		t.Fatal("empty path")
	}
	if filepath.Base(got) != "work_inbox" {
		t.Fatalf("got %q", got)
	}
}

func TestPimalayaPatch(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "config.yaml")
	example, err := ExampleYAML()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, example, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHOULD_I_READ_CONFIG", path)

	gotPath, changed, err := PimalayaPatch("/tmp/sync.toml", "/tmp/store", "gmail")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed")
	}
	if gotPath != path {
		t.Fatalf("path %q", gotPath)
	}
	_, changed2, err := PimalayaPatch("/tmp/sync.toml", "/tmp/store", "gmail")
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Fatal("expected idempotent patch")
	}
}
