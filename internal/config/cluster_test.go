package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestResolveClusterDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := resolveCluster(ClusterFile{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinCosine != defaultClusterMinCosine {
		t.Fatalf("min_cosine: %v", cfg.MinCosine)
	}
	if len(cfg.QuarantineTerms) != 0 {
		t.Fatalf("terms: %v", cfg.QuarantineTerms)
	}
	if cfg.AttentionTrueMin != defaultAttentionTrueMin {
		t.Fatalf("attention_true_min: %v", cfg.AttentionTrueMin)
	}
}

func TestResolveClusterInvalidMinCosine(t *testing.T) {
	t.Parallel()
	_, err := resolveCluster(ClusterFile{MinCosine: 1.5})
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func TestLoadClusterDefaultsFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := `
polypus:
  base_url: "http://127.0.0.1:1320"
`
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("/repo", cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cluster.MinCosine != defaultClusterMinCosine {
		t.Fatalf("min_cosine: %v", cfg.Cluster.MinCosine)
	}
	if cfg.Cluster.AttentionTrueMin != defaultAttentionTrueMin {
		t.Fatalf("attention_true_min: %v", cfg.Cluster.AttentionTrueMin)
	}
}

func TestResolveClusterInvalidAttentionTrueMin(t *testing.T) {
	t.Parallel()
	_, err := resolveCluster(ClusterFile{AttentionTrueMin: 1.5})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveClusterQuarantineTermsLowercase(t *testing.T) {
	t.Parallel()
	cfg, err := resolveCluster(ClusterFile{
		QuarantineTerms: []string{" Changelog ", "job-recommendation"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.QuarantineTerms) != 2 {
		t.Fatalf("terms: %v", cfg.QuarantineTerms)
	}
	if cfg.QuarantineTerms[0] != "changelog" {
		t.Fatalf("term0: %q", cfg.QuarantineTerms[0])
	}
}
