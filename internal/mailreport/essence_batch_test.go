package mailreport

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/behaviorengineering/strop/pkg/openaibatch"
	"github.com/xynova/should-i-read/internal/polypus"
)

type stubEssenceChat struct {
	inflight    atomic.Int32
	maxInflight atomic.Int32
	delay       time.Duration
}

func (s *stubEssenceChat) Chat(context.Context, polypus.ChatRequest) (string, error) {
	cur := s.inflight.Add(1)
	for {
		prev := s.maxInflight.Load()
		if cur <= prev || s.maxInflight.CompareAndSwap(prev, cur) {
			break
		}
	}
	defer s.inflight.Add(-1)
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return "WHY1: w\nWHY2: w\nWHY3: w\nWHY4: w\nWHY5: w\nABOUT: a\nTAGS: fyi\nKIND: notification > software-release > changelog", nil
}

func TestNewParallelChatRunner(t *testing.T) {
	t.Parallel()
	runner, err := NewParallelChatRunner(&stubEssenceChat{}, "m", 4)
	if err != nil || runner == nil {
		t.Fatalf("runner=%v err=%v", runner, err)
	}
	if _, err = NewParallelChatRunner(nil, "m", 4); err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestParallelChatRunnerConcurrent(t *testing.T) {
	t.Parallel()
	stub := &stubEssenceChat{delay: 40 * time.Millisecond}
	runner, err := NewParallelChatRunner(stub, "m", 4)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	lines := make([]openaibatch.ChatLine, 4)
	for i := range lines {
		lines[i] = openaibatch.ChatLine{CustomID: string(rune('a' + i)), Model: "m"}
	}
	got, err := runner.RunChatLines(ctx, lines)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("results=%d", len(got))
	}
	if stub.maxInflight.Load() < 2 {
		t.Fatalf("maxInflight=%d want concurrent chats", stub.maxInflight.Load())
	}
}

type resumeChatLinesStub struct {
	calls atomic.Int32
}

func (s *resumeChatLinesStub) RunChatLines(_ context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error) {
	s.calls.Add(1)
	full := "WHY1: w\nWHY2: w\nWHY3: w\nWHY4: w\nWHY5: w\nABOUT: a\nTAGS: fyi\nKIND: notification > software-release > changelog\n"
	if len(lines) == 1 && len(lines[0].Messages) > 2 {
		msgs := lines[0].Messages
		if msgs[len(msgs)-2][chatRoleKey] != polypus.RoleAssistant {
			return nil, errors.New("expected assistant role")
		}
		if !strings.Contains(msgs[len(msgs)-2][chatContentKey], "WHY1:") {
			return nil, errors.New("expected prior WHY block in assistant turn")
		}
		return map[string]openaibatch.LineResult{lines[0].CustomID: {Content: full}}, nil
	}
	return map[string]openaibatch.LineResult{lines[0].CustomID: {Content: full}}, nil
}

func TestPrefetchOneEssence_resumeAssistantTurn(t *testing.T) {
	whyOnly := "WHY1: w\nWHY2: w\nWHY3: w\nWHY4: w\nWHY5: w\n"
	ln := openaibatch.ChatLine{
		CustomID: "h1",
		Model:    "m",
		Messages: []map[string]string{
			{chatRoleKey: polypus.RoleSystem, chatContentKey: DefaultEssenceSystemPrompt()},
			{chatRoleKey: polypus.RoleUser, chatContentKey: "prompt"},
		},
	}
	stub := &resumeChatLinesStub{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	parsed, err := prefetchOneEssence(ctx, stub, ln, whyOnly, "")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Kind == "" {
		t.Fatal("expected kind")
	}
	if stub.calls.Load() != 1 {
		t.Fatalf("resume chat calls=%d", stub.calls.Load())
	}
}

type escalatePrefetchStub struct {
	calls  atomic.Int32
	models []string
}

func (s *escalatePrefetchStub) RunChatLines(_ context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error) {
	s.calls.Add(1)
	if len(lines) != 1 {
		return nil, errors.New("expected one line")
	}
	s.models = append(s.models, lines[0].Model)
	whyOnly := "WHY1: w\nWHY2: w\nWHY3: w\nWHY4: w\nWHY5: w\n"
	full := whyOnly + "ABOUT: a\nTAGS: fyi\nKIND: notification > software-release > changelog\n"
	content := whyOnly
	if lines[0].Model == "escalate-big" {
		content = full
	}
	return map[string]openaibatch.LineResult{lines[0].CustomID: {Content: content}}, nil
}

type why4RestartPrefetchStub struct {
	calls atomic.Int32
}

func (s *why4RestartPrefetchStub) RunChatLines(_ context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error) {
	s.calls.Add(1)
	if len(lines) != 1 {
		return nil, errors.New("expected one line")
	}
	if len(lines[0].Messages) != 2 {
		return nil, errors.New("expected seed-only messages after WHY1 restart")
	}
	full := "WHY1: w\nWHY2: w\nWHY3: w\nWHY4: w\nWHY5: w\nABOUT: a\nTAGS: fyi\nKIND: notification > software-release > changelog\n"
	return map[string]openaibatch.LineResult{lines[0].CustomID: {Content: full}}, nil
}

func TestPrefetchOneEssence_why4RestartSkipsAssistantAppend(t *testing.T) {
	why4First := "WHY4: delivered promptly\nWHY5: short layout\nTAGS: job-alert\nKIND: notification > job-alert > software-engineering\n"
	ln := openaibatch.ChatLine{
		CustomID: "h1",
		Model:    "m",
		Messages: []map[string]string{
			{chatRoleKey: polypus.RoleSystem, chatContentKey: DefaultEssenceSystemPrompt()},
			{chatRoleKey: polypus.RoleUser, chatContentKey: "prompt"},
		},
	}
	stub := &why4RestartPrefetchStub{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	parsed, err := prefetchOneEssence(ctx, stub, ln, why4First, "")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Kind == "" {
		t.Fatal("expected kind")
	}
	if stub.calls.Load() != 1 {
		t.Fatalf("resume chat calls=%d", stub.calls.Load())
	}
}

func TestPrefetchOneEssence_compensateUsesEscalateModel(t *testing.T) {
	whyOnly := "WHY1: w\nWHY2: w\nWHY3: w\nWHY4: w\nWHY5: w\n"
	ln := openaibatch.ChatLine{
		CustomID: "h1",
		Model:    "micro",
		Messages: []map[string]string{
			{chatRoleKey: polypus.RoleSystem, chatContentKey: DefaultEssenceSystemPrompt()},
			{chatRoleKey: polypus.RoleUser, chatContentKey: "prompt"},
		},
	}
	stub := &escalatePrefetchStub{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	parsed, err := prefetchOneEssence(ctx, stub, ln, whyOnly, "escalate-big")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Kind == "" {
		t.Fatal("expected kind")
	}
	if stub.calls.Load() != essenceParseMaxAttempts {
		t.Fatalf("chat calls=%d", stub.calls.Load())
	}
	if !slicesContains(stub.models, "escalate-big") {
		t.Fatalf("models=%v", stub.models)
	}
}

func slicesContains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
