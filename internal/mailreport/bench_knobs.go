package mailreport

import (
	"fmt"
	"os"
	"strings"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	benchEnvReasoning = "SHOULD_I_READ_BENCH_REASONING"
	benchEnvFeedback  = "SHOULD_I_READ_BENCH_FEEDBACK"

	benchReasoningHigh = "high"
	benchReasoningOff  = "off"

	benchFeedbackChecklist = "checklist"
	benchFeedbackShort     = "short"
)

const semanticShortFeedbackTmpl = "%s cosine %.2f is below %.2f. Rewrite %s using concrete terms from the message. Preserve names, amounts, and causes already stated."

// BenchVariant names for mail bench runs.
const (
	BenchVariantBaseline  = "baseline"
	BenchVariantThinking  = "thinking"
	BenchVariantChecklist = "checklist"
)

// ApplyBenchVariant sets bench env vars for the given variant name.
func ApplyBenchVariant(variant string) error {
	const op = "mailreport.ApplyBenchVariant"
	switch strings.TrimSpace(variant) {
	case BenchVariantBaseline:
		if err := os.Setenv(benchEnvReasoning, benchReasoningOff); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "set bench reasoning")
		}
		if err := os.Setenv(benchEnvFeedback, benchFeedbackShort); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "set bench feedback")
		}
		return nil
	case BenchVariantThinking:
		if err := os.Setenv(benchEnvReasoning, benchReasoningHigh); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "set bench reasoning")
		}
		if err := os.Setenv(benchEnvFeedback, benchFeedbackShort); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "set bench feedback")
		}
		return nil
	case BenchVariantChecklist:
		if err := os.Setenv(benchEnvReasoning, benchReasoningHigh); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "set bench reasoning")
		}
		if err := os.Setenv(benchEnvFeedback, benchFeedbackChecklist); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "set bench feedback")
		}
		return nil
	default:
		return sirerr.New(sirerr.CodeInvalid, op, "unknown bench variant").With("variant", variant)
	}
}

func validateBenchFeedbackEnv() error {
	const op = "mailreport.validateBenchFeedbackEnv"
	v := strings.TrimSpace(os.Getenv(benchEnvFeedback))
	if v == "" || v == benchFeedbackChecklist || v == benchFeedbackShort {
		return nil
	}
	return sirerr.New(sirerr.CodeInvalid, op, "invalid bench feedback override").With("value", v)
}

// essenceEscalationReasoning returns high-effort reasoning unless the bench override disables it.
func essenceEscalationReasoning() (*polypus.ChatReasoning, error) {
	const op = "mailreport.essenceEscalationReasoning"
	v := strings.TrimSpace(os.Getenv(benchEnvReasoning))
	if v == "" || v == benchReasoningHigh {
		return &polypus.ChatReasoning{Effort: polypus.ReasoningEffortHigh}, nil
	}
	if v == benchReasoningOff {
		return nil, nil
	}
	return nil, sirerr.New(sirerr.CodeInvalid, op, "invalid bench reasoning override").With("value", v)
}

func semanticRestartFeedback(field string, score, min float64) string {
	mode := strings.TrimSpace(os.Getenv(benchEnvFeedback))
	if mode == "" || mode == benchFeedbackChecklist {
		return semanticChecklistRestartFeedback(field, score, min)
	}
	if mode == benchFeedbackShort {
		return fmtSemanticShortFeedback(field, score, min)
	}
	// Fail closed at feedback generation time; invalid env should be caught by ApplyBenchVariant.
	return semanticChecklistRestartFeedback(field, score, min)
}

func semanticChecklistRestartFeedback(field string, score, min float64) string {
	return fmt.Sprintf(
		`%s cosine %.2f is below %.2f.

Before writing the final fields, silently validate the candidate:

1. Identify the event the email actually states.
2. Check every name, amount, version, service, and cause against the email.
3. Remove any claim that the email does not support.
4. Prefer a short grounded restatement over an inferred explanation.
5. Preserve concrete facts that are explicitly stated.

Do not output this checklist or your reasoning.

Now emit exactly:
WHY1:
WHY2:
WHY3:
WHY4:
WHY5:
ABOUT:

ABOUT must be one sentence describing the email using only supported facts.`,
		field,
		score,
		min,
	)
}

func fmtSemanticShortFeedback(field string, score, min float64) string {
	return fmt.Sprintf(semanticShortFeedbackTmpl, field, score, min, field)
}
