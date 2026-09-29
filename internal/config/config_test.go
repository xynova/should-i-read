package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"

	"github.com/xynova/should-i-read/internal/oauthcred"
)

func TestExpandString(t *testing.T) {
	t.Parallel()
	resolve := func(name string) (string, error) {
		switch name {
		case "FOO":
			return "bar", nil
		case "EMPTY":
			return "", errors.New("missing")
		default:
			return "", errors.New("missing " + name)
		}
	}

	got, err := ExpandString("pre-${FOO}-post", resolve, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "pre-bar-post" {
		t.Fatalf("got %q", got)
	}

	got, err = ExpandString("${MISSING}", resolve, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	_, err = ExpandString("${MISSING}", resolve, true)
	if err == nil {
		t.Fatal("expected required expand to fail")
	}
}

func TestResolvePathOverrideAndEnv(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(cfgFile, []byte("polypus:\n  base_url: http://127.0.0.1:1320\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ResolvePath(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != cfgFile {
		t.Fatalf("override: got %q", got)
	}

	t.Setenv("SHOULD_I_READ_CONFIG", cfgFile)
	got, err = ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != cfgFile {
		t.Fatalf("env: got %q", got)
	}
}

func TestResolvePathMissingOverrideFailsClosed(t *testing.T) {
	_, err := ResolvePath(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error for missing override path")
	}
}

func TestLoadFromMemKeyring(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
secrets:
  - EMAILOPS_GMAIL_CLIENT_ID
polypus:
  base_url: "http://127.0.0.1:1320"
emailops:
  gmail_client_id: "${EMAILOPS_GMAIL_CLIENT_ID}"
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EMAILOPS_GMAIL_CLIENT_ID", "")

	mem := operatorconfig.NewMemKeyring()
	if err := mem.Set(AppName, "EMAILOPS_GMAIL_CLIENT_ID", "from-keyring-client"); err != nil {
		t.Fatal(err)
	}
	cfg, err := load("/repo", cfgFile, mem)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GmailClientID != "from-keyring-client" {
		t.Fatalf("gmail id: %q", cfg.GmailClientID)
	}
}

func TestLoadExpandsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
polypus:
  base_url: "http://127.0.0.1:1320"
emailops:
  data_dir: "` + filepath.ToSlash(dir) + `/mail"
  gmail_client_id: "${TEST_GMAIL_ID}"
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_GMAIL_ID", "client-from-env")
	t.Setenv("SHOULD_I_READ_CONFIG", "")

	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GmailClientID != "client-from-env" {
		t.Fatalf("gmail id: %q", cfg.GmailClientID)
	}
	if cfg.EmailOpsDataDir != filepath.ToSlash(dir)+"/mail" && cfg.EmailOpsDataDir != filepath.Join(dir, "mail") {
		t.Fatalf("data dir: %q", cfg.EmailOpsDataDir)
	}
	if cfg.EmailOpsRepoPath != filepath.Join("/repo", "providers", "emailops") {
		t.Fatalf("repo path: %q", cfg.EmailOpsRepoPath)
	}
	if cfg.PolypusBaseURL != "http://127.0.0.1:1320" {
		t.Fatalf("polypus: %q", cfg.PolypusBaseURL)
	}
}

func TestLoadUsesOAuthProductEmbed(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
polypus:
  base_url: "http://127.0.0.1:1320"
emailops:
  gmail_client_id: "${EMAILOPS_GMAIL_CLIENT_ID}"
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EMAILOPS_GMAIL_CLIENT_ID", "")

	prev := oauthcred.ProductGmailClientID
	oauthcred.ProductGmailClientID = "product-gmail-client-id"
	t.Cleanup(func() { oauthcred.ProductGmailClientID = prev })

	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GmailClientID != "product-gmail-client-id" {
		t.Fatalf("gmail id: %q", cfg.GmailClientID)
	}
}

func TestLoadPolypusBaseURLFromEnvWhenYAMLEmpty(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
polypus:
  base_url: ""
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envPolypusBaseURL, "http://example.test:9999")
	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PolypusBaseURL != "http://example.test:9999" {
		t.Fatalf("polypus: %q", cfg.PolypusBaseURL)
	}
}

func TestRedacted(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Path:              "/tmp/c.yaml",
		PolypusBaseURL:    "http://127.0.0.1:1320",
		EmailOpsDataDir:   "/data",
		GmailClientID:     "id",
		GmailClientSecret: "sekrit",
		OutlookClientID:   "",
	}
	r := cfg.Redacted()
	emailops, ok := r["emailops"].(map[string]string)
	if !ok {
		t.Fatalf("emailops type: %T", r["emailops"])
	}
	if emailops["gmail_client_id"] != "(set)" {
		t.Fatalf("id: %q", emailops["gmail_client_id"])
	}
	if emailops["gmail_client_secret"] != "(set)" {
		t.Fatalf("secret: %q", emailops["gmail_client_secret"])
	}
	if emailops["outlook_client_id"] != "(unset)" {
		t.Fatalf("outlook: %q", emailops["outlook_client_id"])
	}
	if emailops["data_dir"] != "/data" {
		t.Fatalf("data_dir should not be redacted: %q", emailops["data_dir"])
	}
}
