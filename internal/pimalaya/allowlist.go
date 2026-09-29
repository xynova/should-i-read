package pimalaya

import (
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// Allowed basenames for exec (v1 architecture allow-list).
var allowed = map[string]bool{
	"neverest": true,
	"himalaya": true,
	"ortie":    true,
	"sirup":    true,
}

// AssertAllowed returns an error if binary basename is not on the allow-list.
func AssertAllowed(bin string) error {
	const op = "pimalaya.AssertAllowed"
	base := strings.TrimSpace(bin)
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndex(base, "\\"); i >= 0 {
		base = base[i+1:]
	}
	if !allowed[base] {
		return sirerr.New(sirerr.CodeInvalid, op, "binary not on pimalaya allow-list").With("binary", base)
	}
	return nil
}
