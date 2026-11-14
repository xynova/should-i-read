package maildelete

import (
	"strings"
)

// NormalizeFolder returns the pimdir collection id used for Himalaya -m.
func NormalizeFolder(collection string) string {
	return strings.TrimSpace(collection)
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
