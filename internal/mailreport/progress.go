package mailreport

import (
	"encoding/json"
	"os"

	"github.com/xynova/should-i-read/internal/hoststore"
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

// ProgressEntriesForImport converts JSON progress entries for hoststore import.
func ProgressEntriesForImport(entries map[string]ProgressEntry) map[string]hoststore.ProgressEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]hoststore.ProgressEntry, len(entries))
	for k, v := range entries {
		out[k] = hoststore.ProgressEntry{
			TermID:          v.TermID,
			Path:            v.Path,
			Label:           v.Label,
			Source:          v.Source,
			Strategy:        v.Strategy,
			Kind:            v.Kind,
			About:           v.About,
			Shape:           v.Shape,
			Cosine:          v.Cosine,
			CanonicalTermID: v.CanonicalTermID,
			Reinforced:      v.Reinforced,
			JudgeScore:      v.JudgeScore,
			DraftKind:       v.DraftKind,
			CatalogApplied:  v.CatalogApplied,
			Error:           v.Error,
			SenderTermID:    v.SenderTermID,
			SenderLabel:     v.SenderLabel,
			SenderMapsTo:    v.SenderMapsTo,
			UpdatedAt:       v.UpdatedAt,
		}
	}
	return out
}
