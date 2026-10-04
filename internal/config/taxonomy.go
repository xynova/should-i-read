package config

import (
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const defaultClassifyMax = 50

// TaxonomyFile is the taxonomy YAML section.
type TaxonomyFile struct {
	CatalogPath string `yaml:"catalog_path"`
	ClassifyMax int    `yaml:"classify_max"`
}

// TaxonomyConfig is resolved taxonomy settings.
type TaxonomyConfig struct {
	CatalogPath string
	ClassifyMax int
}

// SeedCatalogPath returns the repo seed vocabulary path.
func SeedCatalogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "inbox-mail.yaml")
}

// DefaultCatalogPath returns the operator-writable catalog path.
func DefaultCatalogPath() (string, error) {
	const op = "config.DefaultCatalogPath"
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vocabularies", "inbox-mail.yaml"), nil
}

func resolveTaxonomy(file TaxonomyFile) (TaxonomyConfig, error) {
	const op = "config.resolveTaxonomy"
	path := strings.TrimSpace(file.CatalogPath)
	if path == "" {
		var err error
		path, err = DefaultCatalogPath()
		if err != nil {
			return TaxonomyConfig{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "default catalog path")
		}
	}
	max := file.ClassifyMax
	if max <= 0 {
		max = defaultClassifyMax
	}
	return TaxonomyConfig{CatalogPath: path, ClassifyMax: max}, nil
}
