package pimdir

import (
	"strings"
)

func keepPreferredSummary(cur, next EmailSummary, haveCur bool) EmailSummary {
	if !haveCur {
		return next
	}
	if summaryRank(next) < summaryRank(cur) {
		return next
	}
	return cur
}

func summaryRank(s EmailSummary) int {
	c := strings.ToLower(strings.TrimSpace(s.Collection))
	if strings.Contains(c, "trash") || strings.Contains(c, "deleted items") {
		return 90
	}
	if c == "inbox" || strings.HasSuffix(c, "/inbox") {
		return 0
	}
	if strings.Contains(c, "important") {
		return 1
	}
	if strings.Contains(c, "all mail") {
		return 10
	}
	return 5
}
