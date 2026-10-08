package mailreport

import (
	"context"

	"github.com/behaviorengineering/strop/pkg/dspy/factory"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

type stropEmbedSeat struct {
	inner factory.Embedder
	model string
}

// NewStropEmbedSeat adapts a strop factory.Embedder to taxonomy harness.Embedder.
func NewStropEmbedSeat(inner factory.Embedder, model string) harness.Embedder {
	if inner == nil {
		return nil
	}
	return &stropEmbedSeat{inner: inner, model: model}
}

func (s *stropEmbedSeat) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	const op = "mailreport.stropEmbedSeat.Embed"
	if s == nil || s.inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil embedder")
	}
	ctx, span := startSeatSpan(ctx, "mailreport.Embed", s.model)
	var retErr error
	defer func() { endSeatSpan(span, retErr) }()
	vecs, err := s.inner.Embed(ctx, texts)
	if err != nil {
		code := sirerr.CodeAI
		if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeUnavailable {
			code = sirerr.CodeUnavailable
		}
		retErr = sirerr.Wrap(err, code, op, "embed")
		return nil, retErr
	}
	return vecs, nil
}
