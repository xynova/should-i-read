package config

import (
	"testing"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

func TestNormalizeTaxonomyStrategy(t *testing.T) {
	walk, err := NormalizeTaxonomyStrategy("")
	if err != nil || walk != string(harness.StrategyWalk) {
		t.Fatalf("empty: got %q err %v", walk, err)
	}
	attach, err := NormalizeTaxonomyStrategy("attach")
	if err != nil || attach != string(harness.StrategyAttach) {
		t.Fatalf("attach: got %q err %v", attach, err)
	}
	_, err = NormalizeTaxonomyStrategy("fly")
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
}

func TestIsAttachStrategy(t *testing.T) {
	if IsAttachStrategy(string(harness.StrategyWalk)) {
		t.Fatal("walk is not attach")
	}
	if !IsAttachStrategy(string(harness.StrategyAttach)) {
		t.Fatal("attach should match")
	}
}
