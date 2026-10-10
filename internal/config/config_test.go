package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
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
oauth:
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
oauth:
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
	if cfg.PolypusBaseURL != "http://127.0.0.1:1320" {
		t.Fatalf("polypus: %q", cfg.PolypusBaseURL)
	}
}

func TestLoadMergesLegacyEmailOpsOAuthSection(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
polypus:
  base_url: "http://127.0.0.1:1320"
emailops:
  gmail_client_id: "${LEGACY_GMAIL_ID}"
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEGACY_GMAIL_ID", "legacy-client")

	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GmailClientID != "legacy-client" {
		t.Fatalf("gmail id: %q", cfg.GmailClientID)
	}
}

func TestMergeOAuthFieldsPrefersOAuthSection(t *testing.T) {
	t.Parallel()
	merged := mergeOAuthFields(File{
		OAuth:          OAuthFile{GmailClientID: "from-oauth"},
		EmailOpsLegacy: OAuthFile{GmailClientID: "from-legacy"},
	})
	if merged.GmailClientID != "from-oauth" {
		t.Fatalf("gmail id: %q", merged.GmailClientID)
	}
}

func TestLoadPolypusBaseURLDefaultWhenUnset(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
polypus:
  base_url: "${POLYPUS_BASE_URL}"
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envPolypusBaseURL, "")
	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PolypusBaseURL != defaultPolypusBaseURL {
		t.Fatalf("polypus: %q", cfg.PolypusBaseURL)
	}
}

func TestLoadNeverestIgnoresEnvWhenYAMLEmpty(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
polypus:
  base_url: "http://127.0.0.1:1320"
pimalaya:
  neverest_bin: ""
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEVEREST_BIN", "/tmp/should-not-use-neverest")
	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pimalaya.NeverestBin != "" {
		t.Fatalf("neverest_bin: %q", cfg.Pimalaya.NeverestBin)
	}
}

func TestDefaultFileTaxonomyWalk(t *testing.T) {
	t.Parallel()
	f := DefaultFile()
	if f.Taxonomy.Strategy != "walk" {
		t.Fatalf("strategy %q", f.Taxonomy.Strategy)
	}
}

func TestLoadPolypusBaseURLFromEnvWhenYAMLEmpty(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
polypus:
  base_url: "${POLYPUS_BASE_URL}"
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
		GmailClientID:     "id",
		GmailClientSecret: "sekrit",
		OutlookClientID:   "",
	}
	r := cfg.Redacted()
	oauth, ok := r["oauth"].(map[string]string)
	if !ok {
		t.Fatalf("oauth type: %T", r["oauth"])
	}
	if oauth["gmail_client_id"] != "(set)" {
		t.Fatalf("id: %q", oauth["gmail_client_id"])
	}
	if oauth["gmail_client_secret"] != "(set)" {
		t.Fatalf("secret: %q", oauth["gmail_client_secret"])
	}
	if oauth["outlook_client_id"] != "(unset)" {
		t.Fatalf("outlook: %q", oauth["outlook_client_id"])
	}
}
