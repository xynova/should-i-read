package config

import (
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	defaultClassifyMax     = 50
	defaultCollectionINBOX = "imap/INBOX"
	defaultAttachMinCosine = 0.80
	defaultWalkReinforce   = 0.70
)

// TaxonomyFile is the taxonomy YAML section.
type TaxonomyFile struct {
	CatalogPath      string   `yaml:"catalog_path"`
	ClassifyMax      int      `yaml:"classify_max"`
	Collections      []string `yaml:"collections"`
	PreAI            *bool    `yaml:"pre_ai"`
	SendersPath      string   `yaml:"senders_path"`
	AuthorOnSkip     *bool    `yaml:"author_on_skip"`
	Strategy         string   `yaml:"strategy"`
	AttachMinCosine  float64  `yaml:"attach_min_cosine"`
	WalkReinforceMin float64  `yaml:"walk_reinforce_min"`
}

// TaxonomyConfig is resolved taxonomy settings.
type TaxonomyConfig struct {
	CatalogPath      string
	ClassifyMax      int
	Collections      []string
	PreAI            bool
	SendersPath      string
	AuthorOnSkip     bool
	Strategy         string
	AttachMinCosine  float64
	WalkReinforceMin float64
}

// SeedCatalogPath returns the repo seed vocabulary path.
func SeedCatalogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "inbox-mail.yaml")
}

// SeedKindCatalogPath returns the repo seed inbox-kind vocabulary path.
func SeedKindCatalogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "inbox-kind.yaml")
}

// SeedSendersPath returns the repo seed senders vocabulary path.
func SeedSendersPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "senders.yaml")
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
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vocabularies", "inbox-kind.yaml"), nil
}

// DefaultSendersPath returns the operator-writable senders catalog path.
func DefaultSendersPath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vocabularies", "senders.yaml"), nil
}

func resolveTaxonomy(file TaxonomyFile) (TaxonomyConfig, error) {
	const op = "config.resolveTaxonomy"
	strategy := strings.TrimSpace(file.Strategy)
	if strategy == "" {
		strategy = "walk"
	}
	if strategy != "walk" && strategy != "attach" {
		return TaxonomyConfig{}, sirerr.New(sirerr.CodeInvalid, op, "taxonomy.strategy must be walk or attach")
	}
	path := strings.TrimSpace(file.CatalogPath)
	if path == "" {
		var err error
		if strategy == "attach" {
			path, err = DefaultKindCatalogPath()
		} else {
			path, err = DefaultCatalogPath()
		}
		if err != nil {
			return TaxonomyConfig{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "default catalog path")
		}
	}
	sendersPath := strings.TrimSpace(file.SendersPath)
	if sendersPath == "" {
		var err error
		sendersPath, err = DefaultSendersPath()
		if err != nil {
			return TaxonomyConfig{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "default senders path")
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
	preAI := true
	if file.PreAI != nil {
		preAI = *file.PreAI
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
	return TaxonomyConfig{
		CatalogPath:      path,
		ClassifyMax:      max,
		Collections:      collections,
		PreAI:            preAI,
		SendersPath:      sendersPath,
		AuthorOnSkip:     authorOnSkip,
		Strategy:         strategy,
		AttachMinCosine:  attachMin,
		WalkReinforceMin: walkMin,
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
