package pimalaya

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// StreamEvent is one Neverest NDJSON progress object (`"event": "..."`).
type StreamEvent struct {
	Name        string `json:"event"`
	Account     string `json:"account"`
	Source      string `json:"source"`
	Endpoint    string `json:"endpoint"`
	Done        int    `json:"done"`
	Total       int    `json:"total"`
	Collections int    `json:"collections"`
}

// ParseStreamEvent decodes a Neverest progress line. The final accounts report is skipped.
func ParseStreamEvent(line []byte) (StreamEvent, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return StreamEvent{}, false
	}
	var ev StreamEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return StreamEvent{}, false
	}
	if strings.TrimSpace(ev.Name) == "" {
		return StreamEvent{}, false
	}
	return ev, true
}

// ProgressLine is the short stripe text for this event. Unknown kinds return empty.
func (e StreamEvent) ProgressLine() string {
	switch strings.TrimSpace(e.Name) {
	case "scan-progressed":
		src := strings.TrimSpace(e.Source)
		if src == "" {
			return fmt.Sprintf("Scanning folders %d/%d", e.Done, e.Total)
		}
		return fmt.Sprintf("Scanning %s %d/%d", src, e.Done, e.Total)
	case "account-started":
		acct := strings.TrimSpace(e.Account)
		if acct == "" {
			return "Syncing…"
		}
		return "Syncing " + acct
	case "listed":
		return fmt.Sprintf("Listed %d folders", e.Collections)
	case "opened":
		ep := strings.TrimSpace(e.Endpoint)
		if ep == "" {
			return "Opened mailbox"
		}
		return "Opened " + ep
	default:
		return ""
	}
}
