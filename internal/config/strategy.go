package config

import (
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// NormalizeTaxonomyStrategy returns walk when empty and rejects unknown values.
func NormalizeTaxonomyStrategy(raw string) (string, error) {
	const op = "config.NormalizeTaxonomyStrategy"
	s := strings.TrimSpace(raw)
	if s == "" {
		return string(harness.StrategyWalk), nil
	}
	walk := string(harness.StrategyWalk)
	attach := string(harness.StrategyAttach)
	if s != walk && s != attach {
		return "", sirerr.New(sirerr.CodeInvalid, op, "taxonomy.strategy must be walk or attach")
	}
	return s, nil
}

// IsAttachStrategy reports whether strategy is taxonomy attach (essence + embed).
func IsAttachStrategy(strategy string) bool {
	return strings.TrimSpace(strategy) == string(harness.StrategyAttach)
}
