package mailoverview

import "strings"

// JoinHashes returns unique, non-empty hashes in first-seen order, comma joined.
func JoinHashes(hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(hashes))
	out := make([]string, 0, len(hashes))
	for _, h := range hashes {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	return strings.Join(out, ",")
}
