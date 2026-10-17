package mailtags

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// Deprecation maps a legacy tag to the preferred replacement.
type Deprecation struct {
	From string `toml:"from"`
	To   string `toml:"to"`
}

type deprecationsFile struct {
	Deprecated []Deprecation `toml:"deprecated"`
}

// LoadDeprecations reads TOML deprecations. Missing file returns nil slice, no error.
func LoadDeprecations(path string) ([]Deprecation, error) {
	const op = "mailtags.LoadDeprecations"
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "read deprecations").With("path", path)
	}
	var f deprecationsFile
	if _, err := toml.Decode(string(b), &f); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode deprecations").With("path", path)
	}
	seenFrom := make(map[string]struct{})
	for i, row := range f.Deprecated {
		from := normalizeToken(row.From)
		to := normalizeToken(row.To)
		if from == "" || to == "" {
			return nil, sirerr.New(sirerr.CodeInvalid, op, "deprecation from and to must be non-empty").With("row", fmt.Sprintf("%d", i+1))
		}
		if _, dup := seenFrom[from]; dup {
			return nil, sirerr.New(sirerr.CodeInvalid, op, "duplicate deprecated from").With("from", from)
		}
		seenFrom[from] = struct{}{}
		f.Deprecated[i].From = from
		f.Deprecated[i].To = to
	}
	return f.Deprecated, nil
}

func deprecationMap(deps []Deprecation) map[string]string {
	out := make(map[string]string, len(deps))
	for _, d := range deps {
		out[d.From] = d.To
	}
	return out
}
