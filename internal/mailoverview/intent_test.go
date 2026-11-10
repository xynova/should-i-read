package mailoverview

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type stubChat struct {
	last polypus.ChatRequest
	out  string
	err  error
}

func (s *stubChat) Chat(_ context.Context, req polypus.ChatRequest) (string, error) {
	s.last = req
	return s.out, s.err
}

func TestCreateIntentGuesserRejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := CreateIntentGuesser(IntentConfig{Model: "m"}); err == nil {
		t.Fatal("expected nil chat error")
	}
	if _, err := CreateIntentGuesser(IntentConfig{Chat: &stubChat{}}); err == nil {
		t.Fatal("expected empty model error")
	}
}

func TestGuessSendsMonthAndSelection(t *testing.T) {
	t.Parallel()
	chat := &stubChat{out: "They are comparing reply tiles."}
	g, err := CreateIntentGuesser(IntentConfig{Chat: chat, Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := g.Guess(ctx, IntentInput{Month: "Oct 2026 1092", Text: "  Need a reply 12  "})
	if err != nil {
		t.Fatal(err)
	}
	if got != "They are comparing reply tiles." {
		t.Fatalf("got %q", got)
	}
	if chat.last.Model != "test-model" {
		t.Fatalf("model %q", chat.last.Model)
	}
	sys := chat.last.Messages[0].Content
	if !strings.Contains(sys, "Speak to the operator as you") || !strings.Contains(sys, "safe to mark deletable") {
		t.Fatalf("system prompt: %q", sys)
	}
	if strings.Contains(strings.ToLower(sys), "they are trying") {
		t.Fatalf("system prompt still talks about they: %q", sys)
	}
	user := chat.last.Messages[1].Content
	if !strings.Contains(user, "This is your inbox.") || !strings.Contains(user, "Month tab: Oct 2026 1092") || !strings.Contains(user, "Need a reply 12") {
		t.Fatalf("user prompt: %q", user)
	}
}

func TestGuessRejectsEmptyAndMissingDeadline(t *testing.T) {
	t.Parallel()
	g, err := CreateIntentGuesser(IntentConfig{Chat: &stubChat{out: "x"}, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := g.Guess(ctx, IntentInput{Text: "   "}); err == nil {
		t.Fatal("expected empty text")
	}
	if _, err := g.Guess(context.Background(), IntentInput{Text: "tiles"}); err == nil {
		t.Fatal("expected missing deadline")
	}
}

func TestGuessWrapsChatFailure(t *testing.T) {
	t.Parallel()
	chat := &stubChat{err: sirerr.New(sirerr.CodeUnavailable, "stub", "down")}
	g, err := CreateIntentGuesser(IntentConfig{Chat: chat, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = g.Guess(ctx, IntentInput{Text: "tiles"})
	if err == nil {
		t.Fatal("expected wrap")
	}
	if c, ok := sirerr.AsCode(err); !ok || c != sirerr.CodeUnavailable {
		t.Fatalf("code %v", err)
	}
}
