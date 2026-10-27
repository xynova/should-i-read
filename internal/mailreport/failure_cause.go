package mailreport

import (
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/tags"
)

// ClassifyFailureCause returns a short stable cause label for a failed message row.
func ClassifyFailureCause(errMsg string) string {
	s := strings.TrimSpace(errMsg)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "not kebab-case"):
		return "kind not kebab-case"
	case strings.Contains(lower, EssenceErrKindAncestor):
		return "kind repeats parent"
	case strings.Contains(lower, EssenceErrKindLong):
		return "kind deeper than 3 levels"
	case strings.Contains(lower, EssenceErrKindShort):
		return "kind not 3 levels"
	case strings.Contains(lower, AssignmentErrPathDeep):
		return "path deeper than 3 levels"
	case strings.Contains(lower, AssignmentErrPathShort):
		return "path shorter than kind"
	case strings.Contains(lower, AssignmentErrPathAncestor):
		return "path repeats parent"
	case strings.Contains(lower, CatalogErrDescriptionLayout):
		return "catalog description layout"
	case strings.Contains(lower, CatalogErrDescriptionShort):
		return "catalog description too short"
	case strings.Contains(lower, "essence semantic gate"):
		return "essence about cosine low"
	case strings.Contains(lower, "dependency rewinds"):
		return "essence semantic repair exhausted"
	case strings.Contains(lower, EssenceErrWhy1Empty):
		return "essence why1 empty"
	case strings.Contains(lower, EssenceErrWhyEmpty):
		return "essence why empty"
	case strings.Contains(lower, EssenceErrAboutOrShape):
		return "essence about/shape empty"
	case strings.Contains(lower, tags.ErrTokenInvalid):
		return "essence tag invalid"
	}
	return truncateFailureCause(s, 60)
}

func truncateFailureCause(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}
