package pimdir

import (
	"strings"
	"unicode"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// RefLooksLikeObjectHash reports whether ref should be treated as a pimdir object hash.
func RefLooksLikeObjectHash(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "@") {
		return false
	}
	if len(ref) < 4 {
		return false
	}
	for _, r := range ref {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// ResolveShowRef maps a CLI ref to summary metadata and an object hash for ReadBlob.
func (r *Reader) ResolveShowRef(ref string) (EmailSummary, string, error) {
	const op = "pimdir.ResolveShowRef"
	if r == nil {
		return EmailSummary{}, "", sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return EmailSummary{}, "", sirerr.New(sirerr.CodeInvalid, op, "empty ref")
	}
	if RefLooksLikeObjectHash(ref) {
		s, err := r.FindByObjectHash(ref)
		if err != nil {
			code, ok := sirerr.AsCode(err)
			if ok && code == sirerr.CodeNotFound {
				return EmailSummary{ObjectHash: ref}, ref, nil
			}
			return EmailSummary{}, "", err
		}
		hash := strings.TrimSpace(s.ObjectHash)
		if hash == "" {
			hash = ref
		}
		return s, hash, nil
	}
	s, err := r.FindByMessageID(ref)
	if err != nil {
		return EmailSummary{}, "", err
	}
	hash := strings.TrimSpace(s.ObjectHash)
	if hash == "" {
		return EmailSummary{}, "", sirerr.New(sirerr.CodeNotFound, op, "message has no stored body yet").
			With("message_id", ref)
	}
	return s, hash, nil
}
