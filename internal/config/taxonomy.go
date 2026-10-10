package config

import (
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	defaultClassifyMax      = 50
	defaultCollectionINBOX  = "imap/INBOX"
	defaultAttachMinCosine  = 0.80
	defaultWalkReinforce    = 0.70
	defaultEssenceBatchSize = 8
	maxEssenceBatchSize     = 16
)

// TaxonomyFile is the taxonomy YAML section.
type TaxonomyFile struct {
	CatalogPath      string   `yaml:"catalog_path"`
	ClassifyMax      int      `yaml:"classify_max"`
	Collections      []string `yaml:"collections"`
	AuthorOnSkip     *bool    `yaml:"author_on_skip"`
	Strategy         string   `yaml:"strategy"`
	MinJudgeScore    float64  `yaml:"min_judge_score"`
	AttachMinCosine  float64  `yaml:"attach_min_cosine"`
	WalkReinforceMin float64  `yaml:"walk_reinforce_min"`
	EssenceBatchSize int      `yaml:"essence_batch_size"`
}

// TaxonomyConfig is resolved taxonomy settings.
type TaxonomyConfig struct {
	CatalogPath      string
	ClassifyMax      int
	Collections      []string
	AuthorOnSkip     bool
	Strategy         string
	MinJudgeScore    float64
	AttachMinCosine  float64
	WalkReinforceMin float64
	EssenceBatchSize int
}

// SeedCatalogPath returns the repo seed vocabulary path.
func SeedCatalogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "inbox-mail.yaml")
}

// SeedKindCatalogPath returns the repo seed inbox-kind vocabulary path.
func SeedKindCatalogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "inbox-kind.yaml")
}

// DefaultCatalogPath returns the operator-writable catalog path.
func DefaultCatalogPath() (string, error) {
	const op = "config.DefaultCatalogPath"
	dir, err := UserConfigDir()
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "user config dir")
	}
	return filepath.Join(dir, "vocabularies", "inbox-mail.yaml"), nil
}

// DefaultKindCatalogPath returns the operator-writable inbox-kind catalog path.
func DefaultKindCatalogPath() (string, error) {
	const op = "config.DefaultKindCatalogPath"
	dir, err := UserConfigDir()
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "user config dir")
	}
	return filepath.Join(dir, "vocabularies", "inbox-kind.yaml"), nil
}

func resolveTaxonomy(file TaxonomyFile) (TaxonomyConfig, error) {
	const op = "config.resolveTaxonomy"
	strategy, err := NormalizeTaxonomyStrategy(file.Strategy)
	if err != nil {
		return TaxonomyConfig{}, err
	}
	path := strings.TrimSpace(file.CatalogPath)
	if path == "" {
		if IsAttachStrategy(strategy) {
			path, err = DefaultKindCatalogPath()
		} else {
			path, err = DefaultCatalogPath()
		}
		if err != nil {
			return TaxonomyConfig{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "default catalog path")
		}
	}
	max := file.ClassifyMax
	if max <= 0 {
		max = defaultClassifyMax
	}
	collections := uniqueNonEmptyStrings(file.Collections)
	if len(collections) == 0 {
		collections = []string{defaultCollectionINBOX}
	}
	authorOnSkip := false
	if file.AuthorOnSkip != nil {
		authorOnSkip = *file.AuthorOnSkip
	}
	attachMin := file.AttachMinCosine
	if attachMin == 0 {
		attachMin = defaultAttachMinCosine
	}
	walkMin := file.WalkReinforceMin
	if walkMin == 0 {
		walkMin = defaultWalkReinforce
	}
	if walkMin > attachMin {
		return TaxonomyConfig{}, sirerr.New(sirerr.CodeInvalid, op, "walk_reinforce_min must be <= attach_min_cosine")
	}
	essenceBatch := file.EssenceBatchSize
	if essenceBatch <= 0 {
		essenceBatch = defaultEssenceBatchSize
	}
	if essenceBatch > maxEssenceBatchSize {
		essenceBatch = maxEssenceBatchSize
	}
	return TaxonomyConfig{
		CatalogPath:      path,
		ClassifyMax:      max,
		Collections:      collections,
		AuthorOnSkip:     authorOnSkip,
		Strategy:         strategy,
		MinJudgeScore:    file.MinJudgeScore,
		AttachMinCosine:  attachMin,
		WalkReinforceMin: walkMin,
		EssenceBatchSize: essenceBatch,
	}, nil
}

func uniqueNonEmptyStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
