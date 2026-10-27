package mailreport

import (
	"context"
	"strings"
	"time"

	stropdspy "github.com/behaviorengineering/strop/pkg/dspy"
	"github.com/behaviorengineering/strop/pkg/dspy/factory"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// polypusEmbedHopTimeout is the homelab Polypus embed hop (timeouts.embed in
// polypus-local envs/homelab/gateway.yaml). The host client must stay longer so a
// stalled hop returns 504 instead of a cancelled request.
const polypusEmbedHopTimeout = 240 * time.Second

// stropEmbedHTTPTimeout is the per-attempt HTTP budget for Polypus embedding via strop CreateLLM.
// Attach sends KIND plus catalog leaves in one POST. Homelab n=32 BGE-M3 batches take about
// 10-15s; a cold 500-leaf POST can take a few minutes. This stays above the 240s Polypus hop
// and below operateTimeoutAttach. Pair with embedRetryMax 0 so retries cannot exhaust operate.
const stropEmbedHTTPTimeout = 270 * time.Second

// StropEmbedFactory creates strop embedders against a gateway base URL.
type StropEmbedFactory interface {
	CreateEmbedder(ctx context.Context, baseURL, modelID string) (factory.Embedder, error)
}

type stropEmbedFactory struct {
	llm *factory.LLMFactory
}

// NewStropEmbedFactory returns a factory for Polypus-compatible embedding.
func NewStropEmbedFactory() StropEmbedFactory {
	return &stropEmbedFactory{llm: factory.NewLLMFactory(nil, stropEmbedHTTPTimeout)}
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
