package mailreport

import (
	"encoding/json"
	"os"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// LoadProgress reads progress from path (empty map if missing).
func LoadProgress(path string) (ProgressFile, error) {
	const op = "mailreport.LoadProgress"
	if path == "" {
		return ProgressFile{Entries: map[string]ProgressEntry{}}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ProgressFile{Entries: map[string]ProgressEntry{}}, nil
		}
		return ProgressFile{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "read progress")
	}
	var pf ProgressFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		return ProgressFile{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode progress")
	}
	if pf.Entries == nil {
		pf.Entries = map[string]ProgressEntry{}
	}
	return pf, nil
}

// SaveProgress writes progress atomically.
func SaveProgress(path string, pf ProgressFile) error {
	const op = "mailreport.SaveProgress"
	if path == "" {
		return nil
	}
	if pf.Entries == nil {
		pf.Entries = map[string]ProgressEntry{}
	}
	raw, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode progress")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write progress tmp")
	}
	if err := os.Rename(tmp, path); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "rename progress")
	}
	return nil
}

func progressNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func progressEntryFromRow(row MessageRow) ProgressEntry {
	return ProgressEntry{
		TermID:          row.TermID,
		Path:            row.Path,
		Label:           row.Label,
		Source:          row.Source,
		Strategy:        row.Strategy,
		Kind:            row.Kind,
		About:           row.About,
		Shape:           row.Shape,
		Cosine:          row.Cosine,
		CanonicalTermID: row.CanonicalTermID,
		Reinforced:      row.Reinforced,
		JudgeScore:      row.JudgeScore,
		DraftKind:       draftKind(row.Draft),
		CatalogApplied:  row.CatalogApplied,
		Error:           row.Error,
		SenderTermID:    row.SenderTermID,
		SenderLabel:     row.SenderLabel,
		SenderMapsTo:    row.SenderMapsTo,
		UpdatedAt:       progressNow(),
	}
}
