package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultHostStorePathEndsWithHostDB(t *testing.T) {
	path, err := DefaultHostStorePath()
	if err != nil {
		t.Fatalf("DefaultHostStorePath: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("host", "should-i-read.db")) {
		t.Fatalf("unexpected path: %q", path)
	}
}

func TestLoadStorePathDefault(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("polypus:\n  base_url: http://127.0.0.1:1320\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	want, err := DefaultHostStorePath()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorePath != want {
		t.Fatalf("StorePath=%q want %q", cfg.StorePath, want)
	}
	red := cfg.Redacted()
	store, ok := red["store"].(map[string]string)
	if !ok || store["path"] != want {
		t.Fatalf("redacted store: %#v", red["store"])
	}
}

func TestLoadStorePathOverride(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := "store:\n  path: /tmp/custom-classify.db\npolypus:\n  base_url: http://127.0.0.1:1320\n"
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorePath != "/tmp/custom-classify.db" {
		t.Fatalf("StorePath=%q", cfg.StorePath)
	}
}
