package mailtags

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const defaultDeprecationsBody = `# Mail tag deprecations. Each [[deprecated]] maps legacy "from" to preferred "to".
# Example:
# [[deprecated]]
# from = "promo"
# to = "promotional"
`

// EnsureFiles creates deprecations when missing. Does not overwrite an existing register.
func EnsureFiles(registerPath, deprecationsPath, seedPath string) error {
	const op = "mailtags.EnsureFiles"
	seedPath = strings.TrimSpace(seedPath)
	if seedPath == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty seed path")
	}
	seed, err := os.ReadFile(seedPath)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "read seed").With("path", seedPath)
	}
	if strings.TrimSpace(string(seed)) == "" {
		return sirerr.New(sirerr.CodeFailed, op, "seed file empty").With("path", seedPath)
	}
	if err := copyIfMissing(deprecationsPath, []byte(defaultDeprecationsBody)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "ensure deprecations")
	}
	registerPath = filepath.Clean(strings.TrimSpace(registerPath))
	if registerPath != "" {
		if _, statErr := os.Stat(registerPath); statErr != nil && !os.IsNotExist(statErr) {
			return sirerr.Wrap(statErr, sirerr.CodeFailed, op, "stat register").With("path", registerPath)
		}
	}
	return nil
}

func copyIfMissing(dest string, body []byte) error {
	const op = "mailtags.copyIfMissing"
	dest = filepath.Clean(strings.TrimSpace(dest))
	if dest == "" {
		return nil
	}
	_, statErr := os.Stat(dest)
	if statErr == nil {
		return nil
	}
	if !os.IsNotExist(statErr) {
		return sirerr.Wrap(statErr, sirerr.CodeFailed, op, "stat dest").With("path", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir dest dir")
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write tmp")
	}
	if err := os.Rename(tmp, dest); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "rename")
	}
	return nil
}
