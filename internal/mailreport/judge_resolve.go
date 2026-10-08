package mailreport

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// JudgeSource records how the SystemOne judge model id was chosen.
type JudgeSource string

const (
	JudgeSourceConfig  JudgeSource = "config"
	JudgeSourceCatalog JudgeSource = "catalog"
	JudgeSourceProbe   JudgeSource = "probe"
)

// SystemOneProber smokes POST /v1/systemone for a model id.
type SystemOneProber interface {
	ProbeSystemOne(ctx context.Context, model string) error
}

// DefaultSystemOneJudgeCandidates returns off-catalog ids when sync omits JEV from /v1/models.
func DefaultSystemOneJudgeCandidates() []string {
	return []string{
		"cf_local/typesafe/jev",
		"typesafe/jev",
	}
}

// ResolveJudgeModel picks a Judge model id and verifies it via SystemOne smoke.
func ResolveJudgeModel(ctx context.Context, prober SystemOneProber, catalogIDs []string, judgeYAML string) (model string, source JudgeSource, err error) {
	const op = "mailreport.ResolveJudgeModel"
	if prober == nil {
		return "", "", sirerr.New(sirerr.CodeInvalid, op, "nil prober")
	}
	if ctx == nil {
		return "", "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", sirerr.New(sirerr.CodeInvalid, op, "context must have a deadline")
	}

	judgeYAML = strings.TrimSpace(judgeYAML)
	if judgeYAML != "" {
		if probeErr := prober.ProbeSystemOne(ctx, judgeYAML); probeErr != nil {
			code := sirerr.CodeUnavailable
			if c, ok := sirerr.AsCode(probeErr); ok {
				code = c
			}
			return "", "", sirerr.Wrap(probeErr, code, op, "configured judge model failed systemone probe").
				With("model", judgeYAML)
		}
		return judgeYAML, JudgeSourceConfig, nil
	}

	tried := map[string]struct{}{}
	for _, id := range catalogIDs {
		if !isJevModelID(id) {
			continue
		}
		tried[id] = struct{}{}
		ok, probeErr := probeJudge(ctx, prober, id)
		if probeErr != nil {
			return "", "", sirerr.Wrap(probeErr, sirerr.CodeUnavailable, op, "judge systemone probe")
		}
		if ok {
			return id, JudgeSourceCatalog, nil
		}
	}

	const maxCandidateProbes = 2
	candidateProbes := 0
	for _, candidate := range DefaultSystemOneJudgeCandidates() {
		if candidateProbes >= maxCandidateProbes {
			break
		}
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, dup := tried[candidate]; dup {
			continue
		}
		tried[candidate] = struct{}{}
		candidateProbes++
		ok, probeErr := probeJudge(ctx, prober, candidate)
		if probeErr != nil {
			return "", "", sirerr.Wrap(probeErr, sirerr.CodeUnavailable, op, "judge systemone probe")
		}
		if ok {
			return candidate, JudgeSourceProbe, nil
		}
	}

	return "", "", sirerr.New(sirerr.CodeUnavailable, op, "no SystemOne judge resolved; set polypus.judge_model, confirm gateway allows typesafe/jev on POST /v1/systemone (JEV may be missing from /v1/models when models.sync is on), run: should-i-read polypus check --classify")
}

// probeJudge returns ok when the probe succeeded; permanent 4xx returns ok=false, err=nil; abort errors return err.
func probeJudge(ctx context.Context, prober SystemOneProber, model string) (ok bool, err error) {
	probeErr := prober.ProbeSystemOne(ctx, model)
	if probeErr == nil {
		return true, nil
	}
	if code, has := sirerr.AsCode(probeErr); has && code == sirerr.CodeAI {
		return false, nil
	}
	return false, probeErr
}

// PickAuthorModel chooses a chat Author model id from config or catalog (no HTTP smoke).
func PickAuthorModel(catalogIDs []string, authorYAML string) (string, error) {
	const op = "mailreport.PickAuthorModel"
	author := strings.TrimSpace(authorYAML)
	if author != "" {
		return author, nil
	}
	candidates := chatAuthorCandidates(catalogIDs)
	if len(candidates) == 0 {
		return "", sirerr.New(sirerr.CodeUnavailable, op, "no classify chat model")
	}
	return candidates[0], nil
}

func isNonChatModelID(id string) bool {
	s := strings.ToLower(strings.TrimSpace(id))
	if s == "" {
		return true
	}
	needles := []string{
		"deepgram",
		"whisper",
		"nova-3",
		"aura-",
		"smart-turn",
		"/flux",
	}
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
