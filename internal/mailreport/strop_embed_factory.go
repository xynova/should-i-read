package mailreport

import (
	"context"
	"strings"
	"time"

	stropdspy "github.com/behaviorengineering/strop/pkg/dspy"
	"github.com/behaviorengineering/strop/pkg/dspy/factory"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// polypusEmbedHopTimeout is the homelab Polypus embed hop default (timeouts.embed)
// when the client omits X-Polypus-Timeout. Chunked POSTs send a per-request header
// from the chunk context deadline.
const polypusEmbedHopTimeout = 60 * time.Second

// stropEmbedHTTPTimeout is the per-attempt HTTP client ceiling for one embeddings POST.
// It must exceed polypusEmbedHopTimeout and embedRequestTimeout(embedChunkSize). Chunking
// embedder applies a shorter per-chunk context deadline; this value is the transport cap.
const stropEmbedHTTPTimeout = 75 * time.Second

// StropEmbedFactory creates strop embedders against a gateway base URL.
type StropEmbedFactory interface {
	CreateEmbedder(ctx context.Context, baseURL, modelID string) (factory.Embedder, error)
}

type stropEmbedFactory struct {
	llm *factory.LLMFactory
}

// NewStropEmbedFactory returns a factory for Polypus-compatible embedding.
func NewStropEmbedFactory() StropEmbedFactory {
	llm := factory.NewLLMFactory(nil, stropEmbedHTTPTimeout)
	llm.SetInstrumentHTTP(polypus.InstrumentEmbedTimeoutHeader)
	return &stropEmbedFactory{llm: llm}
}

func (f *stropEmbedFactory) provider(baseURL, modelID string) stropdspy.ProviderConfig {
	return stropdspy.ProviderConfig{
		APISchema:  "openai",
		BaseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		EmbedModel: strings.TrimSpace(modelID),
		Timeout:    stropEmbedHTTPTimeout.String(),
	}
}

func (f *stropEmbedFactory) CreateEmbedder(ctx context.Context, baseURL, modelID string) (factory.Embedder, error) {
	const op = "mailreport.stropEmbedFactory.CreateEmbedder"
	if f == nil || f.llm == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil factory")
	}
	emb, err := f.llm.CreateEmbedder(ctx, f.provider(baseURL, modelID))
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "strop create embedder")
	}
	return emb, nil
}
