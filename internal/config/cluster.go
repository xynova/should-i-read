package config

import (
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const defaultClusterMinCosine = 0.80

// ClusterFile is the cluster YAML section.
type ClusterFile struct {
	MinCosine        float64  `yaml:"min_cosine"`
	QuarantineTerms  []string `yaml:"quarantine_terms"`
	AttentionTrueMin float64  `yaml:"attention_true_min"`
}

// ClusterConfig is resolved cluster settings.
type ClusterConfig struct {
	MinCosine        float64
	QuarantineTerms  []string
	AttentionTrueMin float64
}

const defaultAttentionTrueMin = 0.5

func resolveCluster(file ClusterFile) (ClusterConfig, error) {
	const op = "config.resolveCluster"
	min := file.MinCosine
	if min == 0 {
		min = defaultClusterMinCosine
	}
	if min < 0 || min > 1 {
		return ClusterConfig{}, sirerr.New(sirerr.CodeInvalid, op, "cluster.min_cosine must be between 0 and 1")
	}
	terms := uniqueNonEmptyStrings(file.QuarantineTerms)
	for i, t := range terms {
		terms[i] = strings.ToLower(strings.TrimSpace(t))
	}
	terms = uniqueNonEmptyStrings(terms)
	atm := file.AttentionTrueMin
	if atm == 0 {
		atm = defaultAttentionTrueMin
	}
	if atm < 0 || atm > 1 {
		return ClusterConfig{}, sirerr.New(sirerr.CodeInvalid, op, "cluster.attention_true_min must be between 0 and 1")
	}
	return ClusterConfig{
		MinCosine:        min,
		QuarantineTerms:  terms,
		AttentionTrueMin: atm,
	}, nil
}
