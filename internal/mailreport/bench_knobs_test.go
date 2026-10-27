package mailreport

import (
	"os"
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/polypus"
)

func clearBenchKnobEnv(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Unsetenv(benchEnvReasoning)
		_ = os.Unsetenv(benchEnvFeedback)
	})
}

func TestEssenceEscalationReasoning_unsetHigh(t *testing.T) {
	clearBenchKnobEnv(t)
	t.Setenv(benchEnvReasoning, "")
	reasoning, err := essenceEscalationReasoning()
	if err != nil {
		t.Fatal(err)
	}
	if reasoning == nil || reasoning.Effort != polypus.ReasoningEffortHigh {
		t.Fatalf("reasoning %+v", reasoning)
	}
}

func TestEssenceEscalationReasoning_off(t *testing.T) {
	clearBenchKnobEnv(t)
	t.Setenv(benchEnvReasoning, benchReasoningOff)
	reasoning, err := essenceEscalationReasoning()
	if err != nil {
		t.Fatal(err)
	}
	if reasoning != nil {
		t.Fatalf("want nil reasoning, got %+v", reasoning)
	}
}

func TestEssenceEscalationReasoning_invalid(t *testing.T) {
	clearBenchKnobEnv(t)
	t.Setenv(benchEnvReasoning, "bogus")
	if _, err := essenceEscalationReasoning(); err == nil {
		t.Fatal("expected error")
	}
}

func TestSemanticRestartFeedback_unsetChecklist(t *testing.T) {
	clearBenchKnobEnv(t)
	t.Setenv(benchEnvFeedback, "")
	got := semanticRestartFeedback("ABOUT", 0.4, 0.55)
	if !strings.Contains(strings.ToLower(got), "silently validate the candidate") {
		t.Fatalf("feedback %q", got)
	}
}

func TestSemanticRestartFeedback_short(t *testing.T) {
	clearBenchKnobEnv(t)
	t.Setenv(benchEnvFeedback, benchFeedbackShort)
	got := semanticRestartFeedback("ABOUT", 0.4, 0.55)
	if strings.Contains(strings.ToLower(got), "silently validate the candidate") {
		t.Fatalf("expected short feedback, got %q", got)
	}
	if !strings.Contains(got, "Rewrite ABOUT") {
		t.Fatalf("feedback %q", got)
	}
}

func TestApplyBenchVariant_baseline(t *testing.T) {
	clearBenchKnobEnv(t)
	if err := ApplyBenchVariant(BenchVariantBaseline); err != nil {
		t.Fatal(err)
	}
	if os.Getenv(benchEnvReasoning) != benchReasoningOff {
		t.Fatalf("reasoning %q", os.Getenv(benchEnvReasoning))
	}
	if os.Getenv(benchEnvFeedback) != benchFeedbackShort {
		t.Fatalf("feedback %q", os.Getenv(benchEnvFeedback))
	}
}

func TestApplyBenchVariant_invalid(t *testing.T) {
	clearBenchKnobEnv(t)
	if err := ApplyBenchVariant("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateBenchFeedbackEnv_invalid(t *testing.T) {
	clearBenchKnobEnv(t)
	t.Setenv(benchEnvFeedback, "bogus")
	if err := validateBenchFeedbackEnv(); err == nil {
		t.Fatal("expected error")
	}
}
