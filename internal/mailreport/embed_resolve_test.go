package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/behaviorengineering/strop/pkg/dspy/factory"
)

type fakeStropEmbedFactory struct {
	okModels map[string]bool
}

func (f *fakeStropEmbedFactory) CreateEmbedder(ctx context.Context, _ string, modelID string) (factory.Embedder, error) {
	if !f.okModels[modelID] {
		return nil, errUnavailable("no model")
	}
	return &fakeFactoryEmbedder{}, nil
}

type fakeFactoryEmbedder struct{}

func (f *fakeFactoryEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = []float64{1, 0}
	}
	return out, nil
}

type errUnavailable string

func (e errUnavailable) Error() string { return string(e) }

func TestResolveEmbedModel_yamlFirst(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fac := &fakeStropEmbedFactory{okModels: map[string]bool{"custom-embed": true}}
	id, err := ResolveEmbedModel(ctx, fac, "http://127.0.0.1:1320", nil, "custom-embed")
	if err != nil || id != "custom-embed" {
		t.Fatalf("%v %q", err, id)
	}
}

func TestResolveEmbedModel_catalogBGE(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fac := &fakeStropEmbedFactory{okModels: map[string]bool{"cf_local/foo-bge": true}}
	id, err := ResolveEmbedModel(ctx, fac, "http://127.0.0.1:1320", []string{"cf_local/foo-bge"}, "")
	if err != nil || id != "cf_local/foo-bge" {
		t.Fatalf("%v %q", err, id)
	}
}
