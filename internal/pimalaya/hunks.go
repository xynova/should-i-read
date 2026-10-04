package pimalaya

import (
	"encoding/json"
	"strings"
)

// HunkRef is one Neverest item patch hunk (IMAP collection + id).
type HunkRef struct {
	Kind       string
	Collection string
	ID         string
}

// FetchedHunks returns fetch hunks from Neverest sync JSON (skips errors and non-fetch).
func FetchedHunks(raw []byte) []HunkRef {
	raw = bytesTrim(raw)
	if len(raw) == 0 {
		return nil
	}
	var doc syncJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	var out []HunkRef
	for _, p := range doc.Item.Patch {
		if len(bytesTrim(p.Error)) > 0 && string(bytesTrim(p.Error)) != "null" {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(p.Hunk.Kind))
		if kind != "fetch" {
			continue
		}
		col := strings.TrimSpace(p.Hunk.Collection)
		id := strings.TrimSpace(p.Hunk.ID)
		if col == "" || id == "" {
			continue
		}
		out = append(out, HunkRef{Kind: kind, Collection: col, ID: id})
	}
	return out
}
