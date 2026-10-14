package pimalaya

import "strings"

// CollectionExcluded reports whether collection matches an exclude name.
// Comparison is ASCII case-insensitive and ignores an optional imap/ prefix.
func CollectionExcluded(collection string, excludes []string) bool {
	got := normalizeCollectionName(collection)
	if got == "" {
		return false
	}
	for _, ex := range excludes {
		if got == normalizeCollectionName(ex) {
			return true
		}
	}
	return false
}

func normalizeCollectionName(name string) string {
	s := strings.TrimSpace(name)
	s = strings.TrimPrefix(s, "imap/")
	s = strings.TrimPrefix(s, "IMAP/")
	return strings.ToLower(s)
}
