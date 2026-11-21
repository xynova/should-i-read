package mailack

import (
	"encoding/json"
	"testing"
)

func TestParseFlagAddReportWrapped(t *testing.T) {
	raw := json.RawMessage(`{"data":{"flags":[{"raw":"\\Seen","iana":"seen"}],"count":1,"queued":true}}`)
	n, flags, err := ParseFlagAddReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || !FlagAddSuccess(n, flags) {
		t.Fatalf("n=%d flags=%v", n, flags)
	}
}

func TestFlagAddSuccessRequiresSeen(t *testing.T) {
	if FlagAddSuccess(1, []string{"flagged"}) {
		t.Fatal("flagged must not count as seen")
	}
	if FlagAddSuccess(0, []string{"seen"}) {
		t.Fatal("zero count must fail")
	}
}
