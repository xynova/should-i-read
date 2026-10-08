package mailreport

import (
	"context"
	"strings"

	stropdspy "github.com/behaviorengineering/strop/pkg/dspy"
	"github.com/behaviorengineering/strop/pkg/dspy/factory"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// StropEmbedFactory creates strop embedders against a gateway base URL.
type StropEmbedFactory interface {
	CreateEmbedder(ctx context.Context, baseURL, modelID string) (factory.Embedder, error)
}

type stropEmbedFactory struct {
	llm *factory.LLMFactory
}

// NewStropEmbedFactory returns a factory for Polypus-compatible embedding.
func NewStropEmbedFactory() StropEmbedFactory {
	return &stropEmbedFactory{llm: factory.NewLLMFactory(nil, 0)}
}

func (f *stropEmbedFactory) provider(baseURL, modelID string) stropdspy.ProviderConfig {
	return stropdspy.ProviderConfig{
		APISchema:  "openai",
		BaseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		EmbedModel: strings.TrimSpace(modelID),
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
