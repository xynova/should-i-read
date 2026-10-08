package mailreport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

type stubProber struct {
	okModels map[string]bool
	err      error
	calls    []string
}

func (s *stubProber) ProbeSystemOne(_ context.Context, model string) error {
	s.calls = append(s.calls, model)
	if s.err != nil {
		return s.err
	}
	if s.okModels[model] {
		return nil
	}
	return sirerr.New(sirerr.CodeAI, "stub", "rejected")
}

func homelabCatalog() []string {
	return []string{
		"@cf/deepgram/aura-2-en",
		"@cf/deepseek-ai/deepseek-v4-flash-0731",
		"@cf/google/gemma-4-26b-a4b-it",
		"cf_local/@cf/google/gemma-4-26b-a4b-it",
		"router/investigator",
	}
}

func ctxProbe(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestResolveJudgeModel_probeCandidate(t *testing.T) {
	p := &stubProber{okModels: map[string]bool{"cf_local/typesafe/jev": true}}
	model, src, err := ResolveJudgeModel(ctxProbe(t), p, homelabCatalog(), "")
	if err != nil {
		t.Fatal(err)
	}
	if model != "cf_local/typesafe/jev" || src != JudgeSourceProbe {
		t.Fatalf("got %q %q", model, src)
	}
}

func TestResolveJudgeModel_configNoFallback(t *testing.T) {
	p := &stubProber{okModels: map[string]bool{"cf_local/typesafe/jev": true}}
	_, _, err := ResolveJudgeModel(ctxProbe(t), p, homelabCatalog(), "bad-chat-model")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(p.calls) != 1 || p.calls[0] != "bad-chat-model" {
		t.Fatalf("calls: %v", p.calls)
	}
}

func TestResolveJudgeModel_abortOnUnavailable(t *testing.T) {
	p := &stubProber{err: sirerr.New(sirerr.CodeUnavailable, "stub", "503")}
	_, _, err := ResolveJudgeModel(ctxProbe(t), p, homelabCatalog(), "")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(p.calls) != 1 {
		t.Fatalf("calls: %v", p.calls)
	}
}

func TestPickAuthorModel_skipsTTS(t *testing.T) {
	author, err := PickAuthorModel(homelabCatalog(), "")
	if err != nil {
		t.Fatal(err)
	}
	if author != "cf_local/@cf/google/gemma-4-26b-a4b-it" {
		t.Fatalf("author: %q", author)
	}
}

func TestPickAuthorModel_yamlOverride(t *testing.T) {
	author, err := PickAuthorModel(homelabCatalog(), "cf_local/@cf/google/gemma-4-26b-a4b-it")
	if err != nil || author != "cf_local/@cf/google/gemma-4-26b-a4b-it" {
		t.Fatalf("author: %q err=%v", author, err)
	}
}

func TestResolveJudgeModel_noDeadline(t *testing.T) {
	p := &stubProber{okModels: map[string]bool{"x": true}}
	_, _, err := ResolveJudgeModel(context.Background(), p, nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIsNonChatModelID(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"@cf/deepgram/aura-2-en", true},
		{"@cf/google/gemma-4-26b-a4b-it", false},
		{"cf_local/typesafe/jev", false},
		{"something/whisper", true},
		{"x/smart-turn-v1", true},
	}
	for _, tc := range cases {
		if isNonChatModelID(tc.id) != tc.want {
			t.Fatalf("%q: got %v want %v", tc.id, !tc.want, tc.want)
		}
	}
}

func TestProbeJudge_permanentVsAbort(t *testing.T) {
	p := &stubProber{okModels: map[string]bool{}}
	ok, err := probeJudge(ctxProbe(t), p, "x")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	p2 := &stubProber{err: errors.New("transport")}
	ok, err = probeJudge(ctxProbe(t), p2, "x")
	if err == nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
