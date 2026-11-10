package mailreport

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// ScoreAttention fills attention on classified messages and persists scores.
func (r *Runner) ScoreAttention(ctx context.Context, art Artifact) (Artifact, error) {
	const op = "mailreport.Runner.ScoreAttention"
	if r == nil {
		return art, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if ctx == nil {
		return art, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if r.store == nil {
		return art, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	tm := r.attentionTrueMin
	if tm == 0 {
		tm = defaultAttentionTrueMin
	}
	art.AttentionTrueMin = tm
	if r.attention == nil {
		return art, nil
	}
	stopUnavailable := false
	promptHash := AttentionPromptHash()
	judgeModel := strings.TrimSpace(r.judgeModel)
	for i := range art.Messages {
		if ctx.Err() != nil {
			return art, ctx.Err()
		}
		row := &art.Messages[i]
		if strings.TrimSpace(row.Error) != "" {
			continue
		}
		hash := strings.TrimSpace(row.ObjectHash)
		if hash == "" {
			continue
		}
		if row.Attention == nil {
			got, getErr := r.store.Get(ctx, hash)
			if getErr == nil && got.AttentionScored {
				row.Attention = AttentionFromState(got.Urgency, got.NeedsReplyNoul, got.InvolvesDecisionNoul, tm)
				continue
			}
		} else {
			continue
		}
		in := AttentionIn{
			Subject: row.Subject,
			Sender:  row.Sender,
			Kind:    row.Kind,
			About:   row.About,
			Shape:   row.Shape,
		}
		if attentionInputEmpty(in) {
			continue
		}
		shapeScores, shapeErr := r.shapeAttentionScores(ctx, *row)
		if shapeErr != nil {
			return art, sirerr.Wrap(shapeErr, sirerr.CodeFailed, op, "shape attention")
		}
		if shapeScores != nil {
			if err := r.persistAttentionHit(ctx, &art, row, hash, *shapeScores, tm, true); err != nil {
				return art, err
			}
			continue
		}
		inputHash := AttentionInputHash(in)
		if judgeModel != "" {
			cached, cacheErr := r.store.GetAttentionInput(ctx, inputHash, judgeModel, promptHash)
			if cacheErr != nil {
				return art, sirerr.Wrap(cacheErr, sirerr.CodeFailed, op, "get attention input cache")
			}
			if cached != nil {
				if err := r.persistAttentionHit(ctx, &art, row, hash, *cached, tm, false); err != nil {
					return art, err
				}
				continue
			}
		}
		if r.aiHops >= r.max {
			art.AttentionOmitted++
			continue
		}
		if stopUnavailable {
			continue
		}
		r.aiHops++
		att, scoreErr := r.attention.Score(ctx, in)
		if scoreErr != nil {
			row.AttentionError = formatRowError(scoreErr)
			if isUnavailable(scoreErr) {
				stopUnavailable = true
			}
			continue
		}
		row.Attention = &att
		if err := r.store.PutAttention(ctx, hash, att.Urgency, att.NeedsReplyNoul, att.InvolvesDecisionNoul); err != nil {
			return art, sirerr.Wrap(err, sirerr.CodeFailed, op, "put attention")
		}
		if judgeModel != "" {
			if err := r.store.PutAttentionInput(ctx, inputHash, judgeModel, promptHash, att.Urgency, att.NeedsReplyNoul, att.InvolvesDecisionNoul); err != nil {
				return art, sirerr.Wrap(err, sirerr.CodeFailed, op, "put attention input cache")
			}
		}
		if err := r.storeShapeAttention(ctx, *row, att); err != nil {
			return art, sirerr.Wrap(err, sirerr.CodeFailed, op, "put shape attention")
		}
		art.AttentionScored++
	}
	return art, nil
}

func (r *Runner) persistAttentionHit(ctx context.Context, art *Artifact, row *MessageRow, hash string, scores hoststore.AttentionInputScores, tm float64, fromShape bool) error {
	const op = "mailreport.Runner.ScoreAttention"
	if art == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil artifact")
	}
	row.Attention = AttentionFromState(scores.Urgency, scores.NeedsReplyNoul, scores.InvolvesDecisionNoul, tm)
	if err := r.store.PutAttention(ctx, hash, scores.Urgency, scores.NeedsReplyNoul, scores.InvolvesDecisionNoul); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "put attention")
	}
	art.AttentionScored++
	if fromShape {
		art.AttentionShapeReused++
	}
	return nil
}

func (r *Runner) shapeAttentionScores(ctx context.Context, row MessageRow) (*hoststore.AttentionInputScores, error) {
	if r == nil || r.store == nil {
		return nil, nil
	}
	_, _, fp, ok := r.shapeFingerprintForItem(Item{Subject: row.Subject, Sender: row.Sender})
	if !ok {
		return nil, nil
	}
	shape, found, err := r.store.GetShape(ctx, fp)
	if err != nil {
		return nil, err
	}
	if !found || !r.shapeRowEligibleForAttention(shape) {
		return nil, nil
	}
	if shape.AttentionScored {
		return &hoststore.AttentionInputScores{
			Urgency:              shape.Urgency,
			NeedsReplyNoul:       shape.NeedsReplyNoul,
			InvolvesDecisionNoul: shape.InvolvesDecisionNoul,
		}, nil
	}
	seed := strings.TrimSpace(shape.SeedObjectHash)
	if seed == "" {
		return nil, nil
	}
	got, getErr := r.store.Get(ctx, seed)
	if getErr != nil || !got.AttentionScored {
		return nil, nil
	}
	if err := r.store.PutShapeAttention(ctx, fp, got.Urgency, got.NeedsReplyNoul, got.InvolvesDecisionNoul); err != nil {
		return nil, err
	}
	return &hoststore.AttentionInputScores{
		Urgency:              got.Urgency,
		NeedsReplyNoul:       got.NeedsReplyNoul,
		InvolvesDecisionNoul: got.InvolvesDecisionNoul,
	}, nil
}

func (r *Runner) storeShapeAttention(ctx context.Context, row MessageRow, att Attention) error {
	if r == nil || r.store == nil {
		return nil
	}
	_, _, fp, ok := r.shapeFingerprintForItem(Item{Subject: row.Subject, Sender: row.Sender})
	if !ok {
		return nil
	}
	shape, found, err := r.store.GetShape(ctx, fp)
	if err != nil {
		return err
	}
	if !found || shape.Status == hoststore.ShapeStatusDisabled {
		return nil
	}
	return r.store.PutShapeAttention(ctx, fp, att.Urgency, att.NeedsReplyNoul, att.InvolvesDecisionNoul)
}

func attentionInputEmpty(in AttentionIn) bool {
	return strings.TrimSpace(in.Subject) == "" &&
		strings.TrimSpace(in.Sender) == "" &&
		strings.TrimSpace(in.Kind) == "" &&
		strings.TrimSpace(in.About) == ""
}

func messageAttentionFromState(got hoststore.StateRow, trueMin float64) *Attention {
	if !got.AttentionScored {
		return nil
	}
	return AttentionFromState(got.Urgency, got.NeedsReplyNoul, got.InvolvesDecisionNoul, trueMin)
}
