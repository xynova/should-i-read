package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBumpOperatorConfigPolypusLocalhost(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
secrets:
  - EMAILOPS_GMAIL_CLIENT_ID
polypus:
  base_url: http://127.0.0.1:1320
emailops:
  gmail_client_id: ${EMAILOPS_GMAIL_CLIENT_ID}
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envConfigPath, cfgFile)

	path, changed, notes, err := BumpOperatorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if path != cfgFile || !changed {
		t.Fatalf("path=%q changed=%v", path, changed)
	}
	if len(notes) < 2 {
		t.Fatalf("notes: %v", notes)
	}
	raw, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "POLYPUS_BASE_URL") || !strings.Contains(text, polypusPlaceholder) {
		t.Fatalf("file:\n%s", raw)
	}
}

func TestBumpOperatorConfigNoOpWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
secrets:
  - POLYPUS_BASE_URL
polypus:
  base_url: "${POLYPUS_BASE_URL}"
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envConfigPath, cfgFile)

	_, changed, _, err := BumpOperatorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no changes")
	}
}
