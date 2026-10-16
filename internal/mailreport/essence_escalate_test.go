package mailreport

import "testing"

func TestEffectiveEssenceEscalateModel(t *testing.T) {
	if got := EffectiveEssenceEscalateModel("micro", ""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	if got := EffectiveEssenceEscalateModel("micro", "micro"); got != "" {
		t.Fatalf("same: %q", got)
	}
	if got := EffectiveEssenceEscalateModel("micro", "large"); got != "large" {
		t.Fatalf("distinct: %q", got)
	}
}
