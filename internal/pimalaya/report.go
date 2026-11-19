package pimalaya

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// FolderCounts is per-IMAP-collection hunk totals from a Neverest sync report.
type FolderCounts struct {
	Collection string
	Fetch      int
	Delete     int
	Other      int
	HunkErrors int
}

// Absence is a Neverest listing gap: IMAP named fewer items than the local replica bound.
type Absence struct {
	Side       string
	Collection string
	Absent     int
	Bound      int
	Rebuild    bool
}

// EngineReport is a summarized Neverest --json stdout payload (not the host ok/data envelope).
type EngineReport struct {
	Kind       string // "sync" | "check" | "init" | "unknown"
	Account    string
	DryRun     bool
	Conflicts  int
	Fetch      int
	Delete     int
	Other      int
	HunkErrors int
	Folders    []FolderCounts
	Absences   []Absence
	Recognized bool
}

type syncJSON struct {
	Account              string        `json:"account"`
	DryRun               bool          `json:"dryRun"`
	OutstandingConflicts int           `json:"outstandingConflicts"`
	Absences             []absenceJSON `json:"absences"`
	Item                 struct {
		Patch []patchEntry `json:"patch"`
	} `json:"item"`
}

type absenceJSON struct {
	Side       string `json:"side"`
	Collection string `json:"collection"`
	Absent     int    `json:"absent"`
	Bound      int    `json:"bound"`
	Rebuild    bool   `json:"rebuild"`
}

type patchEntry struct {
	Hunk struct {
		Kind       string `json:"kind"`
		Side       string `json:"side"`
		Collection string `json:"collection"`
		ID         string `json:"id"`
	} `json:"hunk"`
	Error json.RawMessage `json:"error"`
}

// SummarizeNeverestJSON parses Neverest native JSON into counts. Unrecognized input sets Recognized=false.
func SummarizeNeverestJSON(raw []byte) EngineReport {
	out := EngineReport{Kind: "unknown"}
	raw = extractNeverestSyncDoc(raw)
	if len(raw) == 0 {
		return out
	}
	var doc syncJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return out
	}
	hasPatch := len(doc.Item.Patch) > 0
	hasAccount := strings.TrimSpace(doc.Account) != ""
	hasAbsences := len(doc.Absences) > 0
	if !hasPatch && !hasAccount && !hasAbsences {
		return out
	}
	out.Recognized = true
	out.Kind = "sync"
	out.Account = strings.TrimSpace(doc.Account)
	out.DryRun = doc.DryRun
	out.Conflicts = doc.OutstandingConflicts
	if hasAbsences {
		out.Absences = make([]Absence, 0, len(doc.Absences))
		for _, a := range doc.Absences {
			out.Absences = append(out.Absences, Absence{
				Side:       strings.TrimSpace(a.Side),
				Collection: strings.TrimSpace(a.Collection),
				Absent:     a.Absent,
				Bound:      a.Bound,
				Rebuild:    a.Rebuild,
			})
		}
	}

	folderMap := map[string]*FolderCounts{}
	for _, p := range doc.Item.Patch {
		col := strings.TrimSpace(p.Hunk.Collection)
		if col == "" {
			col = "(unknown)"
		}
		fc := folderMap[col]
		if fc == nil {
			fc = &FolderCounts{Collection: col}
			folderMap[col] = fc
		}
		if len(bytesTrim(p.Error)) > 0 && string(bytesTrim(p.Error)) != "null" {
			fc.HunkErrors++
			out.HunkErrors++
			continue
		}
		switch strings.ToLower(strings.TrimSpace(p.Hunk.Kind)) {
		case "fetch":
			fc.Fetch++
			out.Fetch++
		case "delete":
			fc.Delete++
			out.Delete++
		default:
			fc.Other++
			out.Other++
		}
	}
	out.Folders = make([]FolderCounts, 0, len(folderMap))
	for _, fc := range folderMap {
		out.Folders = append(out.Folders, *fc)
	}
	sort.Slice(out.Folders, func(i, j int) bool {
		return out.Folders[i].Collection < out.Folders[j].Collection
	})
	return out
}

func bytesTrim(b []byte) []byte {
	return bytes.TrimSpace(b)
}

func extractNeverestSyncDoc(raw []byte) []byte {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return raw
	}
	if json.Valid(raw) {
		return unwrapNeverestAccounts(raw)
	}
	lines := bytes.Split(raw, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		if json.Valid(line) {
			return unwrapNeverestAccounts(line)
		}
	}
	return raw
}

func unwrapNeverestAccounts(raw []byte) []byte {
	var wrap struct {
		Accounts []struct {
			Report json.RawMessage `json:"report"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return raw
	}
	if len(wrap.Accounts) != 1 || len(bytes.TrimSpace(wrap.Accounts[0].Report)) == 0 {
		return raw
	}
	return bytes.TrimSpace(wrap.Accounts[0].Report)
}

// HasGmailLabelOverlap reports whether multiple Gmail virtual folders appear in folder counts.
func (r EngineReport) HasGmailLabelOverlap() bool {
	if len(r.Folders) < 2 {
		return false
	}
	gmailFolders := 0
	for _, f := range r.Folders {
		if strings.Contains(f.Collection, "[Gmail]") || f.Collection == "INBOX" {
			gmailFolders++
		}
	}
	return gmailFolders >= 2
}
