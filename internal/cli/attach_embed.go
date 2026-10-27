package cli

import (
	"context"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func newAttachEmbedInner(ctx context.Context, fac mailreport.StropEmbedFactory, baseURL, embedModel string) (harness.Embedder, error) {
	const op = "cli.newAttachEmbedInner"
	stropEmb, err := fac.CreateEmbedder(ctx, baseURL, embedModel)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "create embedder")
	}
	chunked, err := mailreport.CreateChunkingEmbedder(mailreport.ChunkingEmbedderConfig{
		Inner: mailreport.NewStropEmbedSeat(stropEmb, embedModel),
	})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "chunking embedder")
	}
	retried, err := mailreport.CreateRetryingEmbedder(mailreport.RetryingEmbedderConfig{
		Inner: chunked,
	})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "retrying embedder")
	}
	return retried, nil
}
