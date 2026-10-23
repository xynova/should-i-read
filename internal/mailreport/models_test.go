package mailreport

import "testing"

func TestIsSystemOneJudgeID_perplexity(t *testing.T) {
	if !isSystemOneJudgeID("openrouter/perplexity/pplx-decider-v1.2") {
		t.Fatal("expected perplexity walk judge")
	}
}

func TestIsSystemOneJudgeID_notMercury(t *testing.T) {
	if isSystemOneJudgeID("openrouter/inception/mercury-decide") {
		t.Fatal("mercury must not be a walk judge catalog id")
	}
}
