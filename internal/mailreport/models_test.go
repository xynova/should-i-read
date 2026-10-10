package mailreport

import "testing"

func TestPickJudgeAndAuthorModels(t *testing.T) {
	j, a, err := PickJudgeAndAuthorModels(
		[]string{"cf_local/gemma", "cf_local/typesafe/jev"},
		"",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if j != "cf_local/typesafe/jev" {
		t.Fatalf("judge: %q", j)
	}
	if a != "cf_local/gemma" {
		t.Fatalf("author: %q", a)
	}
	j, a, err = PickJudgeAndAuthorModels([]string{"a", "b/jev"}, "yaml-jev", "yaml-chat")
	if err != nil {
		t.Fatal(err)
	}
	if j != "yaml-jev" || a != "yaml-chat" {
		t.Fatalf("yaml override: %q %q", j, a)
	}
}
