package mailreport

import (
	"regexp"
	"strings"
)

// SubjectTemplaterConfig configures subject normalization for shape fingerprints.
type SubjectTemplaterConfig struct{}

// SubjectTemplater replaces volatile tokens in subjects with stable slots.
type SubjectTemplater struct {
	uuidRE    *regexp.Regexp
	hexRE     *regexp.Regexp
	hashTagRE *regexp.Regexp
	versionRE *regexp.Regexp
	longNumRE *regexp.Regexp
}

// CreateSubjectTemplater builds a templater with compiled patterns.
func CreateSubjectTemplater(_ SubjectTemplaterConfig) (*SubjectTemplater, error) {
	uuidRE, err := regexp.Compile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	if err != nil {
		return nil, err
	}
	hexRE, err := regexp.Compile(`(?i)\b[0-9a-f]{7,40}\b`)
	if err != nil {
		return nil, err
	}
	hashTagRE, err := regexp.Compile(`[#!]\d+`)
	if err != nil {
		return nil, err
	}
	versionRE, err := regexp.Compile(`(?i)\bv?\d+\.\d+\.\d+\b`)
	if err != nil {
		return nil, err
	}
	longNumRE, err := regexp.Compile(`\d{4,}`)
	if err != nil {
		return nil, err
	}
	return &SubjectTemplater{
		uuidRE:    uuidRE,
		hexRE:     hexRE,
		hashTagRE: hashTagRE,
		versionRE: versionRE,
		longNumRE: longNumRE,
	}, nil
}

// Template returns a normalized subject template for fingerprinting.
func (t *SubjectTemplater) Template(subject string) string {
	if t == nil {
		return strings.TrimSpace(strings.ToLower(subject))
	}
	s := strings.TrimSpace(strings.ToLower(subject))
	s = t.uuidRE.ReplaceAllString(s, "{id}")
	s = t.hexRE.ReplaceAllString(s, "{sha}")
	s = t.hashTagRE.ReplaceAllString(s, "{n}")
	s = t.versionRE.ReplaceAllString(s, "{version}")
	s = t.longNumRE.ReplaceAllString(s, "{n}")
	return strings.TrimSpace(s)
}

// HasSlot reports whether the template contains a replacement slot.
func HasSlot(template string) bool {
	return strings.Contains(template, "{")
}
