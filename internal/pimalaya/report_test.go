package pimalaya

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarizeNeverestJSON_fixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "sync-gmail-sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := SummarizeNeverestJSON(raw)
	if !r.Recognized {
		t.Fatal("expected recognized sync report")
	}
	if r.Kind != "sync" || r.Account != "gmail" {
		t.Fatalf("account/kind: %+v", r)
	}
	if r.Fetch != 4 || r.Delete != 2 {
		t.Fatalf("totals fetch=%d delete=%d", r.Fetch, r.Delete)
	}
	if r.Conflicts != 0 {
		t.Fatalf("conflicts: %d", r.Conflicts)
	}
	if !r.HasGmailLabelOverlap() {
		t.Fatal("expected gmail overlap hint")
	}
}

func TestSummarizeNeverestJSON_unrecognized(t *testing.T) {
	r := SummarizeNeverestJSON([]byte(`{}`))
	if r.Recognized {
		t.Fatal("empty object should be unrecognized")
	}
	r = SummarizeNeverestJSON([]byte(`not json`))
	if r.Recognized {
		t.Fatal("invalid json should be unrecognized")
	}
}

func TestSummarizeNeverestJSON_ndjsonAbsences(t *testing.T) {
	raw := []byte(`{"event":"account-started","account":"gmail"}
{"accounts":[{"account":"gmail","report":{"account":"gmail","dryRun":true,"item":{"patch":[]},"outstandingConflicts":0,"absences":[{"side":"imap","collection":"INBOX","absent":1054,"bound":2069,"rebuild":false}]}}]}`)
	r := SummarizeNeverestJSON(raw)
	if !r.Recognized {
		t.Fatal("expected recognized accounts report")
	}
	if r.Conflicts != 0 {
		t.Fatalf("conflicts %d", r.Conflicts)
	}
	if len(r.Absences) != 1 {
		t.Fatalf("absences %+v", r.Absences)
	}
	if r.Absences[0].Collection != "INBOX" || r.Absences[0].Absent != 1054 || r.Absences[0].Bound != 2069 {
		t.Fatalf("absence %+v", r.Absences[0])
	}
}
