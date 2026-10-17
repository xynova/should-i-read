package mailtags

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// RenderRegister builds the mail-tag-register.txt body (UTF-8, Unix newlines).
func RenderRegister(counts map[string]int, seedIDs []string, deps []Deprecation) string {
	for _, id := range seedIDs {
		id = normalizeToken(id)
		if id == "" {
			continue
		}
		if _, ok := counts[id]; !ok {
			counts[id] = 0
		}
	}
	var b strings.Builder
	b.WriteString("# Mail tag register (generated). Do not edit by hand.\n")
	b.WriteString("# Active tags from classify_state.shape. Deprecations: edit mail-tag-deprecations.toml\n")
	b.WriteString("\n")
	b.WriteString("## Active tags\n")
	b.WriteString("# tag<TAB>count\n")
	type row struct {
		tag   string
		count int
	}
	var active []row
	for tag, n := range counts {
		if tag == "" {
			continue
		}
		active = append(active, row{tag: tag, count: n})
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].count != active[j].count {
			return active[i].count > active[j].count
		}
		return active[i].tag < active[j].tag
	})
	for _, r := range active {
		fmt.Fprintf(&b, "%s\t%d\n", r.tag, r.count)
	}
	b.WriteString("\n## Deprecated tags\n")
	b.WriteString("# from<TAB>to<TAB>files_still_using_from\n")
	if len(deps) == 0 {
		b.WriteString("# (none)\n")
	} else {
		for _, d := range deps {
			n := counts[d.From]
			fmt.Fprintf(&b, "%s\t%s\t%d\n", d.From, d.To, n)
		}
	}
	return b.String()
}

// CountTagsFromShapes counts valid kebab tokens across stored shape strings (space-separated).
func CountTagsFromShapes(shapes []string) map[string]int {
	counts := make(map[string]int)
	for _, shape := range shapes {
		shape = strings.TrimSpace(shape)
		if shape == "" {
			continue
		}
		for _, tok := range strings.Fields(shape) {
			if !IsValidStoredToken(tok) {
				continue
			}
			tok = normalizeToken(tok)
			counts[tok]++
		}
	}
	return counts
}

// WriteRegister atomically writes the generated register file.
func WriteRegister(path string, shapes []string, seedIDs []string, deps []Deprecation) error {
	const op = "mailtags.WriteRegister"
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty register path")
	}
	counts := CountTagsFromShapes(shapes)
	body := RenderRegister(counts, seedIDs, deps)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir register dir")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write register tmp")
	}
	if err := os.Rename(tmp, path); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "rename register")
	}
	return nil
}
