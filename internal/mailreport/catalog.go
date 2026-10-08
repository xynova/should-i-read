package mailreport

import (
	"os"
	"path/filepath"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// EnsureCatalog copies seed to catalogPath when catalogPath is missing.
func EnsureCatalog(catalogPath, seedPath string) error {
	const op = "mailreport.EnsureCatalog"
	catalogPath = filepath.Clean(catalogPath)
	seedPath = filepath.Clean(seedPath)
	if catalogPath == "" || seedPath == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "catalog and seed paths required")
	}
	_, statErr := os.Stat(catalogPath)
	if statErr == nil {
		return nil
	}
	if !os.IsNotExist(statErr) {
		return sirerr.Wrap(statErr, sirerr.CodeFailed, op, "stat catalog").With("path", catalogPath)
	}
	seed, err := os.ReadFile(seedPath)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "read seed").With("path", seedPath)
	}
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o700); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir catalog dir")
	}
	tmp := catalogPath + ".tmp"
	if err := os.WriteFile(tmp, seed, 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write catalog tmp")
	}
	if err := os.Rename(tmp, catalogPath); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "rename catalog")
	}
	return nil
}

// LoadCatalog reads and builds a taxonomy catalog from path.
func LoadCatalog(path string) (*catalog.Catalog, catalog.Vocabulary, error) {
	const op = "mailreport.LoadCatalog"
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, catalog.Vocabulary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "read catalog").With("path", path)
	}
	vocab, err := catalog.ParseYAML(raw)
	if err != nil {
		return nil, catalog.Vocabulary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "parse catalog")
	}
	cat, err := catalog.BuildCatalog(vocab)
	if err != nil {
		return nil, catalog.Vocabulary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "build catalog")
	}
	return cat, vocab, nil
}

// SaveCatalog atomically writes vocabulary YAML.
func SaveCatalog(path string, vocab catalog.Vocabulary) error {
	const op = "mailreport.SaveCatalog"
	out, err := catalog.SaveYAML(vocab)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode catalog")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write catalog tmp")
	}
	if err := os.Rename(tmp, path); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "rename catalog")
	}
	return nil
}
