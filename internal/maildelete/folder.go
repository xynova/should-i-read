package maildelete

import (
	"strings"
)

// NormalizeFolder strips a leading imap/ namespace for Himalaya -m.
func NormalizeFolder(collection string) string {
	c := strings.TrimSpace(collection)
	c = strings.TrimPrefix(c, "imap/")
	return strings.TrimSpace(c)
}

// IsTrashFolder reports whether collection must not be passed to message delete.
func IsTrashFolder(collection string) bool {
	c := strings.ToLower(strings.TrimSpace(collection))
	if c == "" {
		return false
	}
	c = strings.TrimPrefix(c, "imap/")
	switch c {
	case "trash", "[gmail]/trash":
		return true
	}
	return strings.HasSuffix(c, "/trash")
}
