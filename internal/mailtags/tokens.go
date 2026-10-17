package mailtags

import (
	"regexp"
	"slices"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const essenceErrTagInvalid = "essence tag invalid"

// EssenceErrTagInvalid is the validate message when a tag token fails the gate.
const EssenceErrTagInvalid = essenceErrTagInvalid

var (
	kebabTagRE   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	geometryDeny = map[string]struct{}{
		"rectangular": {},
		"square":      {},
		"circle":      {},
		"triangle":    {},
	}
)

// SplitTags splits a tag field on commas, semicolons, or whitespace.
func SplitTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = normalizeToken(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func normalizeToken(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// JoinTags returns sorted unique tags joined with single spaces.
func JoinTags(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(ids))
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		id = normalizeToken(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	slices.Sort(uniq)
	return strings.Join(uniq, " ")
}

// ValidateToken checks one tag id after normalize (kebab, not geometry, not KIND path).
func ValidateToken(token string) error {
	const op = "mailtags.ValidateToken"
	token = normalizeToken(token)
	if token == "" {
		return sirerr.New(sirerr.CodeAI, op, EssenceErrTagInvalid).With("token", token)
	}
	if strings.Contains(token, ">") {
		return sirerr.New(sirerr.CodeAI, op, EssenceErrTagInvalid).With("token", token)
	}
	if _, denied := geometryDeny[token]; denied {
		return sirerr.New(sirerr.CodeAI, op, EssenceErrTagInvalid).With("token", token)
	}
	if !kebabTagRE.MatchString(token) {
		return sirerr.New(sirerr.CodeAI, op, EssenceErrTagInvalid).With("token", token)
	}
	return nil
}

// IsValidStoredToken is true for kebab tags (used when scanning ledger shapes).
func IsValidStoredToken(token string) bool {
	token = normalizeToken(token)
	if token == "" {
		return false
	}
	if strings.Contains(token, ">") {
		return false
	}
	if _, denied := geometryDeny[token]; denied {
		return false
	}
	return kebabTagRE.MatchString(token)
}
