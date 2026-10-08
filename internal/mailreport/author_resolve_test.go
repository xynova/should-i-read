package mailreport

import (
	"context"
	"testing"

	"github.com/xynova/should-i-read/internal/sirerr"
)

type stubChatter struct {
	okModels map[string]bool
	err      error
	calls    []string
}

func (s *stubChatter) ProbeChat(_ context.Context, model string) error {
	s.calls = append(s.calls, model)
	if s.err != nil {
		return s.err
	}
	if s.okModels[model] {
		return nil
	}
	return sirerr.New(sirerr.CodeAI, "stub", "rejected")
}

func TestResolveAuthorModel_configProbe(t *testing.T) {
	p := &stubChatter{okModels: map[string]bool{"cf_local/gemma": true}}
	model, src, err := ResolveAuthorModel(ctxProbe(t), p, homelabCatalog(), "cf_local/gemma")
	if err != nil || model != "cf_local/gemma" || src != AuthorSourceConfig {
		t.Fatalf("model=%q src=%q err=%v", model, src, err)
	}
	if len(p.calls) != 1 {
		t.Fatalf("calls: %v", p.calls)
	}
}

func TestResolveAuthorModel_configNoFallback(t *testing.T) {
	p := &stubChatter{okModels: map[string]bool{"cf_local/@cf/google/gemma-4-26b-a4b-it": true}}
	_, _, err := ResolveAuthorModel(ctxProbe(t), p, homelabCatalog(), "bad-model")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(p.calls) != 1 || p.calls[0] != "bad-model" {
		t.Fatalf("calls: %v", p.calls)
	}
}

func TestResolveAuthorModel_catalogPrefersCfLocal(t *testing.T) {
	p := &stubChatter{okModels: map[string]bool{"cf_local/@cf/google/gemma-4-26b-a4b-it": true}}
	model, src, err := ResolveAuthorModel(ctxProbe(t), p, homelabCatalog(), "")
	if err != nil {
		t.Fatal(err)
	}
	if model != "cf_local/@cf/google/gemma-4-26b-a4b-it" || src != AuthorSourceCatalog {
		t.Fatalf("model=%q src=%q", model, src)
	}
}

func TestResolveAuthorModel_noDeadline(t *testing.T) {
	p := &stubChatter{okModels: map[string]bool{"x": true}}
	_, _, err := ResolveAuthorModel(context.Background(), p, nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
}
