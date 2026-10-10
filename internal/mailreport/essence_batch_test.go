package mailreport

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/behaviorengineering/strop/pkg/openaibatch"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

func TestNewPreferBatchChatRunner(t *testing.T) {
	t.Parallel()
	runner, err := NewPreferBatchChatRunner(NoopSubLLM(), openaibatch.Client{BaseURL: "http://127.0.0.1:1320"}, "m")
	if err != nil || runner == nil {
		t.Fatalf("runner=%v err=%v", runner, err)
	}
	_, err = NewPreferBatchChatRunner(nil, openaibatch.Client{}, "m")
	if err == nil {
		t.Fatal("expected error for nil sync client")
	}
}

type stubChatLines struct {
	calls atomic.Int32
	last  []openaibatch.ChatLine
}

func (s *stubChatLines) RunChatLines(_ context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error) {
	s.calls.Add(1)
	s.last = lines
	out := map[string]openaibatch.LineResult{}
	for _, ln := range lines {
		out[ln.CustomID] = openaibatch.LineResult{Content: "WHY1: w\nWHY2: w\nWHY3: w\nWHY4: w\nWHY5: w\nABOUT: a\nSHAPE: s\nKIND: notification > software-release > changelog"}
	}
	return out, nil
}

func TestRunner_EssencePrefetchRunChatLines(t *testing.T) {
	stub := &stubChatLines{}
	items := []Item{
		{ObjectHash: "h1", Body: "one"},
		{ObjectHash: "h2", Body: "two"},
		{ObjectHash: "h3", Body: "three"},
	}
	got, errs := prefetchEssence(context.Background(), stub, "m", WorldContextAttach, items, 8)
	if len(errs) != 0 {
		t.Fatalf("errs=%v", errs)
	}
	if stub.calls.Load() != 1 {
		t.Fatalf("calls=%d", stub.calls.Load())
	}
	if len(stub.last) != 3 {
		t.Fatalf("lines=%d", len(stub.last))
	}
	if len(got) != 3 {
		t.Fatalf("parsed=%d", len(got))
	}
	if got["h1"].Kind == "" {
		t.Fatal("expected parsed kind")
	}
	_ = harness.EssenceOut{}
}
