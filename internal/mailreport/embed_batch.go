package mailreport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	// embedChunkSize caps each Polypus embeddings POST (homelab n=32 is ~10-15s).
	embedChunkSize = 32

	embedTimeoutBase    = 8 * time.Second
	embedTimeoutPerText = 450 * time.Millisecond
	embedTimeoutMin     = 12 * time.Second
	embedTimeoutMax     = 45 * time.Second
)

// embedRequestTimeout returns a per-POST deadline from batch size (not a global ceiling).
func embedRequestTimeout(textCount int) time.Duration {
	if textCount < 1 {
		textCount = 1
	}
	d := embedTimeoutBase + embedTimeoutPerText*time.Duration(textCount)
	if d < embedTimeoutMin {
		return embedTimeoutMin
	}
	if d > embedTimeoutMax {
		return embedTimeoutMax
	}
	return d
}

// ChunkingEmbedderConfig splits large Embed calls into bounded batches.
type ChunkingEmbedderConfig struct {
	Inner     harness.Embedder
	ChunkSize int
}

// CreateChunkingEmbedder wraps inner so each HTTP hop embeds at most ChunkSize texts.
func CreateChunkingEmbedder(cfg ChunkingEmbedderConfig) (harness.Embedder, error) {
	const op = "mailreport.CreateChunkingEmbedder"
	if cfg.Inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil inner embedder")
	}
	size := cfg.ChunkSize
	if size < 1 {
		size = embedChunkSize
	}
	return &chunkingEmbedder{inner: cfg.Inner, chunkSize: size}, nil
}

type chunkingEmbedder struct {
	inner     harness.Embedder
	chunkSize int
}

func (c *chunkingEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	const op = "mailreport.chunkingEmbedder.Embed"
	if c == nil || c.inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil embedder")
	}
	if err := ctx.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "embed canceled")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	if len(texts) <= c.chunkSize {
		return c.embedChunk(ctx, texts)
	}
	out := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += c.chunkSize {
		end := start + c.chunkSize
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[start:end]
		vecs, err := c.embedChunk(ctx, chunk)
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	if len(out) != len(texts) {
		return nil, sirerr.New(sirerr.CodeFailed, op, "chunked embed length mismatch")
	}
	return out, nil
}

func (c *chunkingEmbedder) embedChunk(ctx context.Context, texts []string) ([][]float64, error) {
	const op = "mailreport.chunkingEmbedder.embedChunk"
	deadline := embedRequestTimeout(len(texts))
	// chunkCtx deadline is the hop budget; InstrumentEmbedTimeoutHeader maps it to
	// X-Polypus-Timeout on the embeddings POST.
	chunkCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	vecs, err := c.inner.Embed(chunkCtx, texts)
	if err != nil {
		if errorsIsDeadline(err) && ctx.Err() == nil {
			return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "embed chunk deadline").
				With("texts", fmt.Sprintf("%d", len(texts))).
				With("deadline", deadline.String())
		}
		return nil, sirerr.Wrap(err, sirerr.CodeAI, op, "inner embed chunk")
	}
	return vecs, nil
}

func errorsIsDeadline(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
