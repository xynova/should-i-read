package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestResolveTaxonomyDefaultsWalk(t *testing.T) {
	cfg, err := resolveTaxonomy(TaxonomyFile{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Strategy != string(harness.StrategyWalk) {
		t.Fatalf("strategy %q", cfg.Strategy)
	}
	if cfg.MinJudgeScore != 0 {
		t.Fatalf("min judge %v", cfg.MinJudgeScore)
	}
	if !strings.HasSuffix(cfg.CatalogPath, filepath.Join("vocabularies", "inbox-mail.yaml")) {
		t.Fatalf("catalog %q", cfg.CatalogPath)
	}
}

func TestResolveTaxonomyAttachCatalog(t *testing.T) {
	cfg, err := resolveTaxonomy(TaxonomyFile{Strategy: "attach"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.CatalogPath, filepath.Join("vocabularies", "inbox-kind.yaml")) {
		t.Fatalf("catalog %q", cfg.CatalogPath)
	}
}

func TestResolveTaxonomyMinJudgeScorePreserved(t *testing.T) {
	cfg, err := resolveTaxonomy(TaxonomyFile{MinJudgeScore: 0.80})
	if err != nil || cfg.MinJudgeScore != 0.80 {
		t.Fatalf("got %v err %v", cfg.MinJudgeScore, err)
	}
}

func TestResolveTaxonomyEssenceBatchSizeDefault(t *testing.T) {
	cfg, err := resolveTaxonomy(TaxonomyFile{EssenceBatchSize: 0})
	if err != nil || cfg.EssenceBatchSize != defaultEssenceBatchSize {
		t.Fatalf("got %d err %v", cfg.EssenceBatchSize, err)
	}
	cfg, err = resolveTaxonomy(TaxonomyFile{EssenceBatchSize: 99})
	if err != nil || cfg.EssenceBatchSize != maxEssenceBatchSize {
		t.Fatalf("got %d err %v", cfg.EssenceBatchSize, err)
	}
}

func TestResolveTaxonomyDescriptionCosineDefaults(t *testing.T) {
	cfg, err := resolveTaxonomy(TaxonomyFile{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DescriptionCosineMin != defaultDescriptionCosineMin {
		t.Fatalf("min %v", cfg.DescriptionCosineMin)
	}
	if cfg.DescriptionCosineMode != DescriptionCosineModeReport {
		t.Fatalf("mode %q", cfg.DescriptionCosineMode)
	}
	if cfg.EssenceSemanticMaxAttempts != defaultEssenceSemanticMaxAttempts {
		t.Fatalf("attempts %d", cfg.EssenceSemanticMaxAttempts)
	}
}

func TestResolveTaxonomyShapeDefaults(t *testing.T) {
	cfg, err := resolveTaxonomy(TaxonomyFile{})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ShapeConfirm {
		t.Fatal("ShapeConfirm want true")
	}
	if cfg.ShapeMinRepeats != defaultShapeMinRepeats {
		t.Fatalf("min repeats %d", cfg.ShapeMinRepeats)
	}
	if cfg.ShapeConfirmMin != defaultShapeConfirmMin {
		t.Fatalf("confirm min %v", cfg.ShapeConfirmMin)
	}
	wantLeaves := DefaultShapeAllowedLeaves()
	if len(cfg.ShapeAllowedLeaves) != len(wantLeaves) {
		t.Fatalf("leaves %v", cfg.ShapeAllowedLeaves)
	}
}

func TestResolveTaxonomyShapeConfirmFalse(t *testing.T) {
	falseVal := false
	cfg, err := resolveTaxonomy(TaxonomyFile{ShapeConfirm: &falseVal})
	if err != nil || cfg.ShapeConfirm {
		t.Fatalf("got %v err %v", cfg.ShapeConfirm, err)
	}
}

func TestResolveTaxonomyShapeMinRepeatsZero(t *testing.T) {
	cfg, err := resolveTaxonomy(TaxonomyFile{ShapeMinRepeats: 0})
	if err != nil || cfg.ShapeMinRepeats != defaultShapeMinRepeats {
		t.Fatalf("got %d err %v", cfg.ShapeMinRepeats, err)
	}
}

func TestNormalizeDescriptionCosineModeInvalid(t *testing.T) {
	_, err := NormalizeDescriptionCosineMode("bogus")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveTaxonomyUnknownStrategy(t *testing.T) {
	_, err := resolveTaxonomy(TaxonomyFile{Strategy: "fly"})
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code %v ok=%v", code, ok)
	}
}
