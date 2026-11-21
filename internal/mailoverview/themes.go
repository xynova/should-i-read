package mailoverview

import "strings"

// themeOther is the catch-all theme for KIND last slots outside themeBarSlots.
const themeOther = "Other"

// Theme bar slots (last KIND segment) for the eval dashboard.
var themeBarSlots = []string{
	"workflow-failure",
	"pipeline-failure",
	"pull-request",
	"release",
	"engineering",
	"article",
	"payment-failure",
	"invitation",
}

// ThemeCategory maps a KIND last-slot into one of the chart themes or "Other".
func ThemeCategory(lastSlot string) string {
	lastSlot = strings.TrimSpace(lastSlot)
	for _, s := range themeBarSlots {
		if lastSlot == s {
			return lastSlot
		}
	}
	// Fabric version often ends in version under notification
	if lastSlot == "version" {
		return "version"
	}
	return themeOther
}

// KindPathParts splits a KIND string on ">" into trimmed segments.
func KindPathParts(kind string) []string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return nil
	}
	var parts []string
	for _, seg := range splitSeq(kind, '>') {
		t := strings.TrimSpace(seg)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return parts
}

func splitSeq(s string, sep rune) []string {
	var out []string
	var cur []rune
	for _, r := range s {
		if r == sep {
			out = append(out, string(cur))
			cur = cur[:0]
			continue
		}
		cur = append(cur, r)
	}
	out = append(out, string(cur))
	return out
}

func lastKindSegment(kind string) string {
	p := KindPathParts(kind)
	if len(p) == 0 {
		return ""
	}
	return p[len(p)-1]
}

func firstKindSegment(kind string) string {
	p := KindPathParts(kind)
	if len(p) == 0 {
		return ""
	}
	return p[0]
}
