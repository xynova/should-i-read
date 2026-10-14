package mailreport

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	defaultAttentionTrueMin = 0.5
	systemOneQuestionNoul   = "noul"

	attentionUrgencyInstructions  = "Score 0 to 1 how soon the recipient must act. 0 means ignore or FYI. 1 means act now. Do not score whether the mail is unwanted or spam."
	attentionReplyInstructions    = "Score 0 to 1 whether the recipient must send a reply. 0 means no reply. 1 means a reply is required."
	attentionDecisionInstructions = "Score 0 to 1 whether the recipient must choose or approve something. 0 means no decision. 1 means a decision is required."
)

// AttentionIn is input for scoring one message.
type AttentionIn struct {
	Subject string
	Sender  string
	Kind    string
	About   string
	Shape   string
}

// AttentionClient scores message attention.
type AttentionClient interface {
	Score(ctx context.Context, in AttentionIn) (Attention, error)
}

// AttentionScorerConfig configures CreateAttentionScorer.
type AttentionScorerConfig struct {
	Client  *polypus.Client
	Model   string
	TrueMin float64
}

// attentionScorer calls Polypus SystemOne for attention nouls.
type attentionScorer struct {
	client  *polypus.Client
	model   string
	trueMin float64
}

// CreateAttentionScorer builds a production AttentionClient.
func CreateAttentionScorer(cfg AttentionScorerConfig) (AttentionClient, error) {
	const op = "mailreport.CreateAttentionScorer"
	if cfg.Client == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil polypus client")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty judge model")
	}
	tm := cfg.TrueMin
	if tm == 0 {
		tm = defaultAttentionTrueMin
	}
	if tm < 0 || tm > 1 {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "trueMin out of range")
	}
	return &attentionScorer{client: cfg.Client, model: model, trueMin: tm}, nil
}

func (s *attentionScorer) Score(ctx context.Context, in AttentionIn) (Attention, error) {
	const op = "mailreport.attentionScorer.Score"
	if s == nil || s.client == nil {
		return Attention{}, sirerr.New(sirerr.CodeInvalid, op, "nil scorer")
	}
	if ctx == nil {
		return Attention{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return Attention{}, sirerr.New(sirerr.CodeInvalid, op, "missing context deadline")
	}
	state := formatAttentionState(in)
	if strings.TrimSpace(state) == "" {
		return Attention{}, sirerr.New(sirerr.CodeInvalid, op, "empty attention state")
	}
	out, err := s.client.SystemOne(ctx, polypus.SystemOneRequest{
		Model: s.model,
		State: state,
		Questions: map[string]polypus.SystemOneQuestion{
			"urgency":           {Type: systemOneQuestionNoul, Instructions: attentionUrgencyInstructions},
			"needs_reply":       {Type: systemOneQuestionNoul, Instructions: attentionReplyInstructions},
			"involves_decision": {Type: systemOneQuestionNoul, Instructions: attentionDecisionInstructions},
		},
	})
	if err != nil {
		return Attention{}, err
	}
	return parseAttentionAnswers(op, out, s.trueMin)
}

func formatAttentionState(in AttentionIn) string {
	subject := strings.TrimSpace(in.Subject)
	sender := strings.TrimSpace(in.Sender)
	kind := strings.TrimSpace(in.Kind)
	about := strings.TrimSpace(in.About)
	shape := strings.TrimSpace(in.Shape)
	if subject == "" && sender == "" && kind == "" && about == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("Report only. Do not advise deleting, moving, or marking mail.\n\n")
	if subject != "" {
		b.WriteString("Subject: ")
		b.WriteString(truncateRunes(subject, 200))
		b.WriteByte('\n')
	}
	if sender != "" {
		b.WriteString("Sender: ")
		b.WriteString(truncateRunes(sender, 120))
		b.WriteByte('\n')
	}
	if kind != "" {
		b.WriteString("Kind: ")
		b.WriteString(truncateRunes(kind, 200))
		b.WriteByte('\n')
	}
	if about != "" {
		b.WriteString("About: ")
		b.WriteString(truncateRunes(about, 500))
		b.WriteByte('\n')
	}
	if shape != "" {
		b.WriteString("Shape: ")
		b.WriteString(truncateRunes(shape, 120))
		b.WriteByte('\n')
	}
	return b.String()
}

func parseAttentionAnswers(op string, out polypus.SystemOneResponse, trueMin float64) (Attention, error) {
	keys := []string{"urgency", "needs_reply", "involves_decision"}
	vals := make([]float64, 3)
	for i, k := range keys {
		a, ok := out.Answers[k]
		if !ok {
			return Attention{}, sirerr.New(sirerr.CodeAI, op, "missing answer").With("key", k)
		}
		n := a.Noul
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return Attention{}, sirerr.New(sirerr.CodeAI, op, "invalid noul").With("key", k)
		}
		if n < 0 {
			n = 0
		}
		if n > 1 {
			n = 1
		}
		vals[i] = n
	}
	return Attention{
		Urgency:              vals[0],
		NeedsReply:           vals[1] >= trueMin,
		InvolvesDecision:     vals[2] >= trueMin,
		NeedsReplyNoul:       vals[1],
		InvolvesDecisionNoul: vals[2],
	}, nil
}

// AttentionFromState maps stored nouls to Attention using trueMin.
func AttentionFromState(urgency, needsReplyNoul, involvesDecisionNoul float64, trueMin float64) *Attention {
	if trueMin == 0 {
		trueMin = defaultAttentionTrueMin
	}
	return &Attention{
		Urgency:              urgency,
		NeedsReply:           needsReplyNoul >= trueMin,
		InvolvesDecision:     involvesDecisionNoul >= trueMin,
		NeedsReplyNoul:       needsReplyNoul,
		InvolvesDecisionNoul: involvesDecisionNoul,
	}
}

// RollupAttention aggregates member attention onto a cluster.
func RollupAttention(members []MessageRow, _ float64) *Attention {
	var maxUrgency float64
	var needsReply, involvesDecision bool
	var any bool
	for _, m := range members {
		if strings.TrimSpace(m.Error) != "" || m.Attention == nil {
			continue
		}
		any = true
		if m.Attention.Urgency > maxUrgency {
			maxUrgency = m.Attention.Urgency
		}
		if m.Attention.NeedsReply {
			needsReply = true
		}
		if m.Attention.InvolvesDecision {
			involvesDecision = true
		}
	}
	if !any {
		return nil
	}
	return &Attention{
		Urgency:          maxUrgency,
		NeedsReply:       needsReply,
		InvolvesDecision: involvesDecision,
	}
}

// SortClustersByAttention orders clusters for operator display.
func SortClustersByAttention(clusters []ClusterRow) {
	sort.Slice(clusters, func(i, j int) bool {
		ui := clusterUrgencySortKey(clusters[i])
		uj := clusterUrgencySortKey(clusters[j])
		if ui != uj {
			return ui > uj
		}
		if clusters[i].Size != clusters[j].Size {
			return clusters[i].Size > clusters[j].Size
		}
		return clusters[i].ID < clusters[j].ID
	})
}

func clusterUrgencySortKey(c ClusterRow) float64 {
	if c.Attention == nil {
		return -1
	}
	return c.Attention.Urgency
}

// stubAttentionClient is for tests.
type stubAttentionClient struct {
	fn func(ctx context.Context, in AttentionIn) (Attention, error)
}

func (s *stubAttentionClient) Score(ctx context.Context, in AttentionIn) (Attention, error) {
	if s == nil || s.fn == nil {
		return Attention{}, sirerr.New(sirerr.CodeInvalid, "stubAttentionClient.Score", "nil fn")
	}
	return s.fn(ctx, in)
}
