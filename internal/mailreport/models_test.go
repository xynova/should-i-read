package mailreport

import "testing"

func TestPickJudgeAndAuthorModels(t *testing.T) {
	j, a := PickJudgeAndAuthorModels(
		[]string{"cf_local/gemma", "cf_local/typesafe/jev"},
		"",
		"",
	)
	if j != "cf_local/typesafe/jev" {
		t.Fatalf("judge: %q", j)
	}
	if a != "cf_local/gemma" {
		t.Fatalf("author: %q", a)
	}
	j, a = PickJudgeAndAuthorModels([]string{"a", "b/jev"}, "yaml-jev", "yaml-chat")
	if j != "yaml-jev" || a != "yaml-chat" {
		t.Fatalf("yaml override: %q %q", j, a)
	}
}
