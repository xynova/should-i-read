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
		if r.aiHops >= r.max {
			art.AttentionOmitted++
			continue
		}
		if stopUnavailable {
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
		art.AttentionScored++
	}
	return art, nil
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
