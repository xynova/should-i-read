package mailreport

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// WorthReadPrior is a report-only estimate from operator outcomes.
type WorthReadPrior struct {
	Score     float64 `json:"score"`
	Basis     string  `json:"basis"`
	Outcome   string  `json:"outcome,omitempty"`
	Neighbors int     `json:"neighbors,omitempty"`
	SampleN   int     `json:"sample_n,omitempty"`
}

// ScoreWorthReadPrior adds worth_read_prior on messages when prior is enabled and outcomes exist.
func (r *Runner) ScoreWorthReadPrior(ctx context.Context, art Artifact) (Artifact, error) {
	const op = "mailreport.Runner.ScoreWorthReadPrior"
	if r == nil || !r.worthReadPrior {
		return art, nil
	}
	if ctx == nil {
		return art, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if r.store == nil {
		art.WorthReadError = "nil store"
		return art, nil
	}
	embedModel := strings.TrimSpace(art.EmbedModel)
	if embedModel == "" {
		embedModel = strings.TrimSpace(r.embedModel)
	}
	allRows, err := r.store.ListOutcomes(ctx)
	if err != nil {
		art.WorthReadError = err.Error()
		return art, nil
	}
	if len(allRows) == 0 {
		return art, nil
	}
	outcomes := map[string]hoststore.MessageOutcome{}
	for _, row := range allRows {
		outcomes[row.ObjectHash] = row
	}
	allWithKind, err := r.store.ListOutcomesWithKind(ctx)
	if err != nil {
		art.WorthReadError = err.Error()
		return art, nil
	}
	kindRates := buildKindSenderRates(allWithKind, r.worthReadMinNeighbors)
	minCosine := r.worthReadMinCosine
	if minCosine <= 0 {
		minCosine = r.attachMinCosine
	}
	for i := range art.Messages {
		row := &art.Messages[i]
		hash := strings.TrimSpace(row.ObjectHash)
		if hash == "" {
			continue
		}
		if p := exactWorthPrior(outcomes[hash]); p != nil {
			row.WorthReadPrior = p
			continue
		}
		if embedModel != "" {
			if p, nerr := neighborWorthPrior(ctx, r.store, hash, embedModel, outcomes, r.worthReadSimilarK, minCosine, r.worthReadMinNeighbors); nerr != nil {
				art.WorthReadError = nerr.Error()
				return art, nil
			} else if p != nil {
				row.WorthReadPrior = p
				continue
			}
		}
		if p := kindSenderWorthPrior(row, kindRates, hash, r.worthReadMinNeighbors); p != nil {
			row.WorthReadPrior = p
		}
	}
	return art, nil
}

func exactWorthPrior(o hoststore.MessageOutcome) *WorthReadPrior {
	switch strings.TrimSpace(o.Outcome) {
	case hoststore.OutcomeTrashedAtSource, hoststore.OutcomeAlreadyInTrash:
		return &WorthReadPrior{Score: 0, Basis: "exact", Outcome: o.Outcome}
	case hoststore.OutcomeKept:
		return &WorthReadPrior{Score: 1, Basis: "exact", Outcome: o.Outcome}
	default:
		return nil
	}
}

func neighborWorthPrior(
	ctx context.Context,
	st *hoststore.Store,
	hash, embedModel string,
	outcomes map[string]hoststore.MessageOutcome,
	k int,
	minCosine float64,
	minNeighbors int,
) (*WorthReadPrior, error) {
	neighbors, err := st.Similar(ctx, hash, embedModel, k)
	if err != nil {
		return nil, err
	}
	var kept, total int
	for _, n := range neighbors {
		if n.ObjectHash == hash || n.Cosine < minCosine {
			continue
		}
		o, ok := outcomes[n.ObjectHash]
		if !ok {
			continue
		}
		if code := strings.TrimSpace(o.Outcome); code == hoststore.OutcomeKept {
			kept++
			total++
		} else if code == hoststore.OutcomeTrashedAtSource || code == hoststore.OutcomeAlreadyInTrash {
			total++
		}
	}
	if total < minNeighbors {
		return nil, nil
	}
	return &WorthReadPrior{
		Score:     float64(kept) / float64(total),
		Basis:     "neighbors",
		Neighbors: total,
		SampleN:   total,
	}, nil
}

type kindSenderRate struct {
	kept  int
	total int
}

func buildKindSenderRates(rows []hoststore.OutcomeWithKind, minNeighbors int) map[string]kindSenderRate {
	out := map[string]kindSenderRate{}
	_ = minNeighbors
	for _, row := range rows {
		kind := strings.TrimSpace(row.Kind)
		domain := SenderDomain(row.Sender)
		if kind == "" || domain == "" {
			continue
		}
		key := kind + "\x00" + domain
		r := out[key]
		switch strings.TrimSpace(row.Outcome) {
		case hoststore.OutcomeKept:
			r.kept++
			r.total++
		case hoststore.OutcomeTrashedAtSource, hoststore.OutcomeAlreadyInTrash:
			r.total++
		}
		out[key] = r
	}
	return out
}

func kindSenderWorthPrior(row *MessageRow, rates map[string]kindSenderRate, selfHash string, minNeighbors int) *WorthReadPrior {
	kind := strings.TrimSpace(row.Kind)
	if kind == "" {
		kind = strings.TrimSpace(row.TermID)
	}
	domain := SenderDomain(row.Sender)
	if kind == "" || domain == "" {
		return nil
	}
	key := kind + "\x00" + domain
	r, ok := rates[key]
	if !ok || r.total < minNeighbors {
		return nil
	}
	_ = selfHash
	return &WorthReadPrior{
		Score:   float64(r.kept) / float64(r.total),
		Basis:   "kind_sender",
		SampleN: r.total,
	}
}
