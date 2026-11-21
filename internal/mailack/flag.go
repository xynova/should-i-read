package mailack

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// FlagAdder runs one Himalaya flag add.
type FlagAdder interface {
	FlagAdd(ctx context.Context, folder, envelopeID, flag string) (*pimalaya.Result, error)
}

type flagAddReport struct {
	Count int `json:"count"`
	Flags []struct {
		Raw  string `json:"raw"`
		Iana string `json:"iana"`
	} `json:"flags"`
	Queued bool `json:"queued"`
}

// ParseFlagAddReport extracts count and flags from Himalaya flag add --json.
func ParseFlagAddReport(raw json.RawMessage) (int, []string, error) {
	const op = "mailack.ParseFlagAddReport"
	if len(raw) == 0 {
		return 0, nil, sirerr.New(sirerr.CodeFailed, op, "empty json")
	}
	var envelope struct {
		Data flagAddReport `json:"data"`
	}
	var report flagAddReport
	if err := json.Unmarshal(raw, &envelope); err == nil && (envelope.Data.Count > 0 || len(envelope.Data.Flags) > 0) {
		report = envelope.Data
	} else {
		if err := json.Unmarshal(raw, &report); err != nil {
			return 0, nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "parse flag add report")
		}
	}
	names := make([]string, 0, len(report.Flags))
	for _, f := range report.Flags {
		if n := strings.TrimSpace(f.Iana); n != "" {
			names = append(names, n)
			continue
		}
		if n := strings.TrimSpace(f.Raw); n != "" {
			names = append(names, n)
		}
	}
	return report.Count, names, nil
}

// FlagAddSuccess reports whether Himalaya wrote the seen flag.
func FlagAddSuccess(count int, flags []string) bool {
	if count < 1 {
		return false
	}
	for _, f := range flags {
		n := strings.ToLower(strings.TrimSpace(f))
		if n == pimalaya.FlagSeen || n == `\seen` || n == "\\seen" {
			return true
		}
	}
	return false
}
