package mailreport

import (
	"context"
	"math"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
)

const (
	// StrategyShapeConfirm is the classify_state strategy for Jev-confirmed shape copies.
	StrategyShapeConfirm = "shape-confirm"
	shapeChoiceMatch     = "match"
	shapeChoiceMismatch  = "mismatch"
)

const shapeConfirmWorld = "Report-only: confirm whether this message is the same recurring event type as the known template. Do not suggest deleting or moving mail."

type shapeConfirmDecision struct {
	shape       hoststore.MailShapeRow
	fingerprint string
	score       float64
}

func (r *Runner) shapeConfirmEnabled(source string) bool {
	if r == nil || !r.shapeConfirm || source == SourceBench {
		return false
	}
	if r.seats.Judge == nil || r.subjectTemplater == nil {
		return false
	}
	return true
}

func (r *Runner) currentEssencePromptHash() string {
	if r != nil && strings.TrimSpace(r.currentPromptHash) != "" {
		return r.currentPromptHash
	}
	return EssencePromptHash(DefaultEssenceSystemPrompt())
}

func (r *Runner) shapeFingerprintForItem(item Item) (domain, template, fingerprint string, ok bool) {
	if r == nil || r.subjectTemplater == nil {
		return "", "", "", false
	}
	domain = senderDomain(item.Sender)
	if domain == "" {
		return "", "", "", false
	}
	template = r.subjectTemplater.Template(item.Subject)
	if !HasSlot(template) {
		return "", "", "", false
	}
	fp := ShapeFingerprint(domain, template)
	return domain, template, fp, fp != ""
}

func (r *Runner) lastSlotAllowed(lastSlot string) bool {
	slot := strings.TrimSpace(lastSlot)
	if slot == "" {
		return false
	}
	for _, leaf := range r.shapeAllowedLeaves {
		if strings.TrimSpace(leaf) == slot {
			return true
		}
	}
	return false
}

func (r *Runner) shapeRowEligible(row hoststore.MailShapeRow) bool {
	if row.Status != hoststore.ShapeStatusActive {
		return false
	}
	if row.PromptHash != r.currentEssencePromptHash() {
		return false
	}
	return r.lastSlotAllowed(row.LastSlot)
}

func (r *Runner) planShapeSkips(ctx context.Context, pending []Item) (essenceItems []Item, skipByHash map[string]shapeConfirmDecision) {
	skipByHash = map[string]shapeConfirmDecision{}
	type candidate struct {
		item  Item
		shape hoststore.MailShapeRow
		fp    string
	}
	var candidates []candidate
	for _, item := range pending {
		_, _, fp, ok := r.shapeFingerprintForItem(item)
		if !ok {
			essenceItems = append(essenceItems, item)
			continue
		}
		row, found, err := r.store.GetShape(ctx, fp)
		if err != nil || !found || !r.shapeRowEligible(row) {
			essenceItems = append(essenceItems, item)
			continue
		}
		candidates = append(candidates, candidate{item: item, shape: row, fp: fp})
	}
	for _, c := range candidates {
		dec, ok := r.jevShapeConfirm(ctx, c.item, c.shape, c.fp)
		if ok {
			skipByHash[strings.TrimSpace(c.item.ObjectHash)] = dec
			continue
		}
		if storeErr := r.store.RejectShape(ctx, c.fp); storeErr != nil {
			// logged via art later if needed
			_ = storeErr
		}
		essenceItems = append(essenceItems, c.item)
	}
	return essenceItems, skipByHash
}

func (r *Runner) jevShapeConfirm(ctx context.Context, item Item, shape hoststore.MailShapeRow, fp string) (shapeConfirmDecision, bool) {
	if r.seats.Judge == nil {
		return shapeConfirmDecision{}, false
	}
	in := harness.DecideIn{
		Text: formatItemText(item) + "\nKnown shape: " + shape.SubjectTemplate +
			" from " + shape.SenderDomain + ". Proposed KIND: " + shape.Kind + ".",
		WorldContext: shapeConfirmWorld,
		Options: []harness.PackedOption{
			{Choice: shapeChoiceMatch, Label: "match", Description: "same recurring event; copy the proposed KIND"},
			{Choice: shapeChoiceMismatch, Label: "mismatch", Description: "different event; run full Essence"},
			{Choice: harness.ChoiceSkip, Description: "not sure"},
		},
	}
	out, err := r.seats.Judge.Decide(ctx, in)
	if err != nil {
		return shapeConfirmDecision{}, false
	}
	if out.Choice != shapeChoiceMatch {
		return shapeConfirmDecision{}, false
	}
	if math.IsNaN(out.Score) || math.IsInf(out.Score, 0) || out.Score < r.shapeConfirmMin {
		return shapeConfirmDecision{}, false
	}
	return shapeConfirmDecision{shape: shape, fingerprint: fp, score: out.Score}, true
}

func (r *Runner) midWaveShapeDecision(ctx context.Context, item Item) (shapeConfirmDecision, bool) {
	_, _, fp, ok := r.shapeFingerprintForItem(item)
	if !ok {
		return shapeConfirmDecision{}, false
	}
	row, found, err := r.store.GetShape(ctx, fp)
	if err != nil || !found || !r.shapeRowEligible(row) {
		return shapeConfirmDecision{}, false
	}
	return r.jevShapeConfirm(ctx, item, row, fp)
}

func (r *Runner) rowFromShapeConfirm(item Item, dec shapeConfirmDecision) (MessageRow, bool) {
	row := MessageRow{
		ObjectHash:      item.ObjectHash,
		Subject:         item.Subject,
		Sender:          item.Sender,
		Kind:            dec.shape.Kind,
		TermID:          dec.shape.TermID,
		CanonicalTermID: dec.shape.CanonicalTermID,
		About:           strings.TrimSpace(item.Subject),
		Source:          StrategyShapeConfirm,
		Strategy:        StrategyShapeConfirm,
		JudgeScore:      dec.score,
	}
	if rt, ok := r.cat.PreferLeaf(dec.shape.TermID); ok && rt != nil {
		row.TermID = rt.ID
		row.Label = rt.Label
		row.Path = rt.Path
		if row.CanonicalTermID == "" {
			row.CanonicalTermID = rt.ID
		}
	} else {
		return MessageRow{}, false
	}
	syncRowPathFromCatalog(r.cat, &row)
	alignAssignmentPathWithKind(&row)
	clipCatalogTermToKindDepth(r.cat, &row)
	if pathErr := validateAssignmentPath(row); pathErr != nil {
		return MessageRow{}, false
	}
	if !assignmentComplete(row) {
		return MessageRow{}, false
	}
	return row, true
}

func (r *Runner) persistShapeConfirm(ctx context.Context, hash, token string, item Item, dec shapeConfirmDecision, art *Artifact) error {
	row, ok := r.rowFromShapeConfirm(item, dec)
	if !ok {
		return r.operateAndPersist(ctx, hash, token, item, art)
	}
	storeCtx, storeCancel := storeWriteContext(ctx)
	defer storeCancel()
	emb, embErr := r.canonicalEmbedding(ctx, row)
	if embErr != nil {
		row.Error = formatRowError(embErr)
		art.Messages = append(art.Messages, row)
		if failErr := r.store.Fail(storeCtx, hash, token, row.Error); failErr != nil {
			return failErr
		}
		return nil
	}
	if err := r.store.Complete(storeCtx, hash, token, stateFromRow(row, r.vocab.ID, r.judgeModel, r.embedModel), emb); err != nil {
		return err
	}
	if err := r.store.ConfirmShape(storeCtx, dec.fingerprint); err != nil {
		art.ShapeStoreError = err.Error()
	}
	art.Messages = append(art.Messages, row)
	art.Classified++
	art.NewlyClassified++
	art.ShapeConfirmed++
	return nil
}

func (r *Runner) maybePromoteShape(ctx context.Context, item Item, row MessageRow, art *Artifact) {
	if r == nil || r.store == nil || r.subjectTemplater == nil {
		return
	}
	if row.Strategy == StrategyShapeConfirm {
		return
	}
	domain, template, fp, ok := r.shapeFingerprintForItem(item)
	if !ok {
		return
	}
	segs := catalog.ParseKindSegments(strings.TrimSpace(row.Kind))
	if len(segs) == 0 {
		return
	}
	lastSlot := strings.TrimSpace(segs[len(segs)-1])
	if !r.lastSlotAllowed(lastSlot) {
		return
	}
	fpEssence := essenceFingerprint(r.essenceModel, r.essenceEscalateModel, DefaultEssenceSystemPrompt())
	if _, hit, err := r.store.GetEssence(ctx, item.ObjectHash, fpEssence); err != nil || !hit {
		return
	}
	hit := hoststore.ShapeHit{
		Fingerprint:     fp,
		Domain:          domain,
		Template:        template,
		Kind:            strings.TrimSpace(row.Kind),
		TermID:          strings.TrimSpace(row.TermID),
		CanonicalTermID: strings.TrimSpace(row.CanonicalTermID),
		LastSlot:        lastSlot,
		PromptHash:      r.currentEssencePromptHash(),
		EssenceModel:    r.essenceModel,
		SeedObjectHash:  strings.TrimSpace(item.ObjectHash),
		MinRepeats:      r.shapeMinRepeats,
	}
	storeCtx, cancel := storeWriteContext(ctx)
	defer cancel()
	res, err := r.store.RecordShapeHit(storeCtx, hit)
	if err != nil {
		art.ShapeStoreError = err.Error()
		return
	}
	if res.Promoted {
		art.ShapesPromoted++
	}
}
