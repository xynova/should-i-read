package mailreport

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const essenceSystemPrompt = `Describe what the message is. Do not decide keep, discard, or unwanted.

Reply with these keys (one per line). Values may continue on the next line after "KEY:".

WHY1:
WHY2:
WHY3:
WHY4:
WHY5:
ABOUT:
SHAPE:
KIND:

KIND must be 3 to 5 kebab-case segments joined by " > " (example: notification > software-release > changelog).`

// WorldContextAttach is the fixed brief for Attach Operate.
const WorldContextAttach = "Report only. Describe what the message is. Do not decide keep, discard, or unwanted."

var essenceKeyRE = regexp.MustCompile(`^(WHY[1-5]|ABOUT|SHAPE|KIND):\s*(.*)$`)

func parseEssenceResponse(raw string) (harness.EssenceOut, error) {
	var why [5]string
	about, shape, kind := "", "", ""
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var pendingKey string
	var pendingVal []string
	flush := func() {
		if pendingKey == "" {
			return
		}
		val := strings.TrimSpace(strings.Join(pendingVal, "\n"))
		switch pendingKey {
		case "WHY1":
			why[0] = val
		case "WHY2":
			why[1] = val
		case "WHY3":
			why[2] = val
		case "WHY4":
			why[3] = val
		case "WHY5":
			why[4] = val
		case "ABOUT":
			about = val
		case "SHAPE":
			shape = val
		case "KIND":
			kind = val
		}
		pendingKey = ""
		pendingVal = nil
	}
	for _, line := range lines {
		if m := essenceKeyRE.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			flush()
			pendingKey = m[1]
			if rest := strings.TrimSpace(m[2]); rest != "" {
				pendingVal = []string{rest}
			}
			continue
		}
		if pendingKey != "" && strings.TrimSpace(line) != "" {
			pendingVal = append(pendingVal, strings.TrimSpace(line))
		}
	}
	flush()
	out := harness.EssenceOut{Why: why, About: about, Shape: shape, Kind: kind}
	if err := validateParsedEssence(out); err != nil {
		return harness.EssenceOut{}, err
	}
	return out, nil
}

func validateParsedEssence(e harness.EssenceOut) error {
	const op = "mailreport.validateParsedEssence"
	for i, w := range e.Why {
		if strings.TrimSpace(w) == "" {
			return sirerr.New(sirerr.CodeAI, op, fmt.Sprintf("why%d empty", i+1))
		}
	}
	if strings.TrimSpace(e.About) == "" || strings.TrimSpace(e.Shape) == "" {
		return sirerr.New(sirerr.CodeAI, op, "about or shape empty")
	}
	segs := catalog.ParseKindSegments(e.Kind)
	if len(segs) < 3 {
		return sirerr.New(sirerr.CodeAI, op, "kind needs 3+ segments")
	}
	return nil
}
