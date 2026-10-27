package mailreport

import (
	"context"
	"strings"

	"github.com/behaviorengineering/strop/pkg/evaluation"
	"github.com/behaviorengineering/strop/pkg/orchestration"
	"github.com/behaviorengineering/strop/pkg/streaming"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	essencePhaseWhy  orchestration.PhaseID = "essence_why"
	essencePhaseTags orchestration.PhaseID = "essence_tags"
	essencePhaseKind orchestration.PhaseID = "essence_kind"

	essenceFieldWhyBlock = "why_block"
	essenceFieldShape    = "shape"
	essenceFieldKind     = "kind"
)

type budgetedPhaseWalk struct {
	inner orchestration.CompositionStrategy
	max   int
}

func (b budgetedPhaseWalk) Phases() []orchestration.PhaseDef { return b.inner.Phases() }

func (b budgetedPhaseWalk) RunPhase(ctx context.Context, phase orchestration.PhaseDef, feedback string, ch streaming.EventChannel) (*orchestration.PhaseResult, error) {
	return b.inner.RunPhase(ctx, phase, feedback, ch)
}

func (b budgetedPhaseWalk) Result() (*orchestration.CompositionResult, error) {
	return b.inner.Result()
}

func (b budgetedPhaseWalk) MaxCompositionRewinds() int { return b.max }

func (b budgetedPhaseWalk) InvalidateFrom(phaseID orchestration.PhaseID) {
	if inv, ok := b.inner.(orchestration.CompositionDraftInvalidator); ok {
		inv.InvalidateFrom(phaseID)
	}
}

type essencePhaseWalkRunner struct {
	e                   *polypusEssencer
	messageText         string
	mailPrefix          string
	prompts             EssencePrompts
	pendingFeedback     string
	bestWhyBlock        string
	bestAbout           string
	bestAboutScore      float64
	lastGatedAboutScore float64
	gatedAbout          bool
	escalationAttempted bool
	escalating          bool
}

func (r *essencePhaseWalkRunner) Run(ctx context.Context, req orchestration.PhaseWalkRequest, _ streaming.EventChannel) (*orchestration.PhaseWalkResponse, error) {
	const op = "mailreport.essencePhaseWalkRunner.Run"
	if r == nil || r.e == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	switch orchestration.PhaseID(req.PhaseID) {
	case essencePhaseWhy:
		return r.runWhy(ctx, req)
	case essencePhaseTags:
		return r.runTags(ctx, req.LockedOutput)
	case essencePhaseKind:
		return r.runKind(ctx, req.LockedOutput)
	default:
		return nil, sirerr.New(sirerr.CodeInvalid, op, "unknown essence phase")
	}
}

func (r *essencePhaseWalkRunner) runWhy(ctx context.Context, req orchestration.PhaseWalkRequest) (*orchestration.PhaseWalkResponse, error) {
	const op = "mailreport.essencePhaseWalkRunner.runWhy"
	user := r.mailPrefix
	feedback := strings.TrimSpace(req.PreviousFeedback)
	if feedback == "" {
		feedback = strings.TrimSpace(r.pendingFeedback)
	}
	r.pendingFeedback = ""
	if feedback != "" {
		user = user + "\n\n" + feedback
	}
	shell := r.e.whyShell
	if feedback != "" && r.e.whyEscalateShell != nil {
		r.escalating = true
		shell = r.e.whyEscalateShell
	}
	whyRaw, err := shell.Run(ctx, r.prompts.Why, user)
	if err != nil {
		return nil, essenceShellErr(op, err)
	}
	whyOut, parseErr := parseEssenceWhy(whyRaw)
	if parseErr != nil {
		return nil, sirerr.Wrap(parseErr, sirerr.CodeAI, op, "essence parse")
	}
	block := formatEssenceWhyBlock(whyOut)
	block = r.keepBestWhy(ctx, whyOut, block)
	return &orchestration.PhaseWalkResponse{
		OutputFields: map[string]string{
			essenceFieldWhyBlock: block,
		},
		Passed: true,
		Score:  1,
	}, nil
}

func (r *essencePhaseWalkRunner) keepBestWhy(ctx context.Context, whyOut harness.EssenceOut, block string) string {
	if r == nil || r.e == nil || r.e.embedder == nil {
		return block
	}
	score, err := MeasureDescriptionCosine(ctx, r.e.embedder, r.messageText, whyOut.About)
	if err != nil {
		return block
	}
	if r.bestWhyBlock == "" {
		r.bestWhyBlock = block
		r.bestAbout = whyOut.About
		r.bestAboutScore = score
		return block
	}
	kept, keptScore := keepBetterText(r.bestAbout, r.bestAboutScore, whyOut.About, score)
	if kept != whyOut.About {
		return r.bestWhyBlock
	}
	r.bestWhyBlock = block
	r.bestAbout = whyOut.About
	r.bestAboutScore = keptScore
	return block
}

func (r *essencePhaseWalkRunner) runTags(ctx context.Context, locked map[string]string) (*orchestration.PhaseWalkResponse, error) {
	const op = "mailreport.essencePhaseWalkRunner.runTags"
	whyOut, err := essenceWhyFromLocked(locked)
	if err != nil {
		return nil, err
	}
	tagsUser := r.mailPrefix + "\n\n" + formatEssenceWhyBlock(whyOut)
	shell := r.e.tagsShell
	if r.escalating && r.e.tagsEscalateShell != nil {
		shell = r.e.tagsEscalateShell
	}
	tagsRaw, runErr := shell.Run(ctx, r.prompts.Tags, tagsUser)
	if runErr != nil {
		return nil, essenceShellErr(op, runErr)
	}
	shape, parseErr := parseEssenceTags(tagsRaw, r.e.tagCatalog)
	if parseErr != nil {
		return nil, sirerr.Wrap(parseErr, sirerr.CodeAI, op, "essence parse")
	}
	return &orchestration.PhaseWalkResponse{
		OutputFields: map[string]string{essenceFieldShape: shape},
		Passed:       true,
		Score:        1,
	}, nil
}

func (r *essencePhaseWalkRunner) runKind(ctx context.Context, locked map[string]string) (*orchestration.PhaseWalkResponse, error) {
	const op = "mailreport.essencePhaseWalkRunner.runKind"
	whyOut, err := essenceWhyFromLocked(locked)
	if err != nil {
		return nil, err
	}
	shape := strings.TrimSpace(locked[essenceFieldShape])
	tagsUser := r.mailPrefix + "\n\n" + formatEssenceWhyBlock(whyOut)
	kindUser := tagsUser + "\nTAGS: " + shape
	shell := r.e.kindShell
	if r.escalating && r.e.kindEscalateShell != nil {
		shell = r.e.kindEscalateShell
	}
	kindRaw, runErr := shell.Run(ctx, r.prompts.Kind, kindUser)
	if runErr != nil {
		return nil, essenceShellErr(op, runErr)
	}
	kind, parseErr := parseEssenceKind(kindRaw)
	if parseErr != nil {
		return nil, sirerr.Wrap(parseErr, sirerr.CodeAI, op, "essence parse")
	}
	out := harness.EssenceOut{
		Why:   whyOut.Why,
		About: whyOut.About,
		Shape: shape,
		Kind:  kind,
	}
	if err := validateParsedEssence(out); err != nil {
		return &orchestration.PhaseWalkResponse{
			Passed:   false,
			Feedback: err.Error(),
		}, nil
	}
	return r.kindSemanticResult(ctx, out, kind), nil
}

func (r *essencePhaseWalkRunner) kindSemanticResult(ctx context.Context, out harness.EssenceOut, kind string) *orchestration.PhaseWalkResponse {
	pass := &orchestration.PhaseWalkResponse{
		OutputFields: map[string]string{essenceFieldKind: kind},
		Passed:       true,
		Score:        1,
	}
	restart := semanticRestartPhase(ctx, r.e, r.messageText, out)
	if restart == "" {
		return pass
	}
	if r.escalationAttempted || r.e.whyEscalateShell == nil {
		return pass
	}
	score, scoreErr := MeasureDescriptionCosine(ctx, r.e.embedder, r.messageText, out.About)
	if scoreErr != nil {
		return pass
	}
	r.escalationAttempted = true
	if r.gatedAbout && score <= r.lastGatedAboutScore {
		return pass
	}
	r.gatedAbout = true
	r.lastGatedAboutScore = score
	min := r.e.descriptionCosineMin
	if min <= 0 {
		min = 0.55
	}
	r.pendingFeedback = semanticRestartFeedback("ABOUT", score, min)
	return &orchestration.PhaseWalkResponse{
		OutputFields: map[string]string{essenceFieldKind: kind},
		Passed:       false,
		Feedback:     r.pendingFeedback,
		RestartAt:    restart,
	}
}

func essenceWhyFromLocked(locked map[string]string) (harness.EssenceOut, error) {
	const op = "mailreport.essenceWhyFromLocked"
	if locked == nil {
		return harness.EssenceOut{}, sirerr.New(sirerr.CodeFailed, op, "missing locked why")
	}
	block := strings.TrimSpace(locked[essenceFieldWhyBlock])
	if block == "" {
		return harness.EssenceOut{}, sirerr.New(sirerr.CodeFailed, op, "incomplete why phase")
	}
	out, err := parseEssenceWhy(block)
	if err != nil {
		return harness.EssenceOut{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "parse locked why")
	}
	return out, nil
}

func essenceShellErr(op string, runErr error) error {
	if runErr == nil {
		return nil
	}
	return sirerr.Wrap(runErr, sirerr.CodeAI, op, "essence chat")
}

func semanticRestartPhase(ctx context.Context, e *polypusEssencer, messageText string, out harness.EssenceOut) orchestration.PhaseID {
	if e == nil || e.descriptionCosineMode != config.DescriptionCosineModeRepair {
		return ""
	}
	if e.embedder == nil {
		return ""
	}
	min := e.descriptionCosineMin
	if min <= 0 {
		min = 0.55
	}
	aboutScore, err := MeasureDescriptionCosine(ctx, e.embedder, messageText, out.About)
	if err == nil && aboutScore < min {
		return essencePhaseWhy
	}
	return ""
}

func keepBetterText(prev string, prevScore float64, next string, nextScore float64) (string, float64) {
	if strings.TrimSpace(prev) == "" {
		return next, nextScore
	}
	if nextScore < prevScore {
		return prev, prevScore
	}
	return next, nextScore
}

func (e *polypusEssencer) essencePhaseWalk(ctx context.Context, messageText, mailPrefix string, prompts EssencePrompts) (harness.EssenceOut, error) {
	const op = "mailreport.polypusEssencer.essencePhaseWalk"
	runner := &essencePhaseWalkRunner{
		e:           e,
		messageText: messageText,
		mailPrefix:  mailPrefix,
		prompts:     prompts,
	}
	inner := orchestration.NewPhaseWalkStrategy(orchestration.PhaseWalkConfig{
		Phases: []orchestration.PhaseDef{
			{ID: essencePhaseWhy, DisplayName: "essence why", MaxAttempts: 1},
			{ID: essencePhaseTags, DisplayName: "essence tags", MaxAttempts: 1},
			{ID: essencePhaseKind, DisplayName: "essence kind", MaxAttempts: 1},
		},
		OwnedFields: orchestration.PhaseWalkOwnedFields{
			essencePhaseWhy:  {essenceFieldWhyBlock},
			essencePhaseTags: {essenceFieldShape},
			essencePhaseKind: {essenceFieldKind},
		},
		Runner:  runner,
		Version: 1,
		Finalize: func(draft map[string]string, _ []evaluation.LabeledEval, _ []float64) (*orchestration.CompositionResult, error) {
			whyOut, err := parseEssenceWhy(strings.TrimSpace(draft[essenceFieldWhyBlock]))
			if err != nil {
				return nil, err
			}
			out := harness.EssenceOut{
				Why:   whyOut.Why,
				About: whyOut.About,
				Shape: strings.TrimSpace(draft[essenceFieldShape]),
				Kind:  strings.TrimSpace(draft[essenceFieldKind]),
			}
			if err := validateParsedEssence(out); err != nil {
				return nil, err
			}
			return &orchestration.CompositionResult{OutputState: out}, nil
		},
	})
	maxRewinds := e.essenceSemanticMaxAttempts
	if maxRewinds <= 0 {
		maxRewinds = 2
	}
	strategy := budgetedPhaseWalk{inner: inner, max: maxRewinds}
	res, err := orchestration.RunCompositionLoop(ctx, strategy, nil)
	if err != nil {
		return harness.EssenceOut{}, sirerr.Wrap(err, sirerr.CodeAI, op, "composition loop")
	}
	if res == nil || res.OutputState == nil {
		return harness.EssenceOut{}, sirerr.New(sirerr.CodeFailed, op, "empty composition result")
	}
	out, ok := res.OutputState.(harness.EssenceOut)
	if !ok {
		return harness.EssenceOut{}, sirerr.New(sirerr.CodeFailed, op, "unexpected composition output type")
	}
	return out, nil
}
