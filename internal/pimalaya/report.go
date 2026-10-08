package pimalaya

import (
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
	Recognized bool
}

type syncJSON struct {
	Account              string `json:"account"`
	DryRun               bool   `json:"dryRun"`
	OutstandingConflicts int    `json:"outstandingConflicts"`
	Item                 struct {
		Patch []patchEntry `json:"patch"`
	} `json:"item"`
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
	raw = bytesTrim(raw)
	if len(raw) == 0 {
		return out
	}
	var doc syncJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return out
	}
	hasPatch := len(doc.Item.Patch) > 0
	hasAccount := strings.TrimSpace(doc.Account) != ""
	if !hasPatch && !hasAccount {
		return out
	}
	out.Recognized = true
	out.Kind = "sync"
	out.Account = strings.TrimSpace(doc.Account)
	out.DryRun = doc.DryRun
	out.Conflicts = doc.OutstandingConflicts

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
	return []byte(strings.TrimSpace(string(b)))
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
