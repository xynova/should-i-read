package mailreport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
)

// EssencePromptHash fingerprints the essence system prompt contract.
func EssencePromptHash(systemPrompt string) string {
	sys := strings.TrimSpace(systemPrompt)
	if sys == "" {
		sys = DefaultEssenceSystemPrompt()
	}
	sum := sha256.Sum256([]byte(sys))
	return hex.EncodeToString(sum[:])
}

func essenceFingerprint(model, escalateModel, systemPrompt string) hoststore.EssenceFingerprint {
	return hoststore.EssenceFingerprint{
		EssenceModel:  strings.TrimSpace(model),
		EscalateModel: EffectiveEssenceEscalateModel(model, escalateModel),
		PromptHash:    EssencePromptHash(systemPrompt),
	}
}

func essenceWhyToStore(why [5]string) string {
	b, err := json.Marshal(why)
	if err != nil {
		return ""
	}
	return string(b)
}

func essenceWhyFromStore(raw string) [5]string {
	var why [5]string
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return why
	}
	_ = json.Unmarshal([]byte(raw), &why)
	return why
}

func essenceOutFromRow(r hoststore.EssenceRow) harness.EssenceOut {
	return harness.EssenceOut{
		Kind:  r.Kind,
		About: r.About,
		Shape: r.Shape,
		Why:   essenceWhyFromStore(r.Why),
	}
}

func essenceRowFromOut(out harness.EssenceOut) hoststore.EssenceRow {
	return hoststore.EssenceRow{
		Kind:  out.Kind,
		About: out.About,
		Shape: out.Shape,
		Why:   essenceWhyToStore(out.Why),
	}
}
