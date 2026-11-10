package mailoverview

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	intentMaxSelected = 8000
	intentSystem      = "This is the operator's own inbox, not a third person's. Speak to the operator as you. The operator selected a region to decide if that mail is noise. Reply in two short parts: a one-sentence TLDR of the selection, then whether it looks safe to mark deletable or why it should stay. Stay with the selected text. Do not claim any mailbox change happened. Do not mention being a model."
)

// ChatClient posts a Polypus chat completion.
type ChatClient interface {
	Chat(ctx context.Context, req polypus.ChatRequest) (string, error)
}

// IntentConfig builds an IntentGuesser.
type IntentConfig struct {
	Chat  ChatClient
	Model string
}

// IntentGuesser asks Polypus what a selected overview region appears to ask.
type IntentGuesser struct {
	chat  ChatClient
	model string
}

// CreateIntentGuesser returns a guesser after required fields are set.
func CreateIntentGuesser(cfg IntentConfig) (*IntentGuesser, error) {
	const op = "mailoverview.CreateIntentGuesser"
	if cfg.Chat == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil chat client")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "model is empty")
	}
	return &IntentGuesser{chat: cfg.Chat, model: model}, nil
}

// IntentInput is one overview selection.
type IntentInput struct {
	Month string
	Text  string
}

// Guess returns a short intent reading of the selection.
func (g *IntentGuesser) Guess(ctx context.Context, in IntentInput) (string, error) {
	const op = "mailoverview.IntentGuesser.Guess"
	if g == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil guesser")
	}
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", sirerr.New(sirerr.CodeInvalid, op, "context must have a deadline")
	}
	selected := clipSelected(in.Text)
	if selected == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "selected text is empty")
	}
	user := formatIntentUser(in.Month, selected)
	out, err := g.chat.Chat(ctx, polypus.ChatRequest{
		Model:       g.model,
		Temperature: 0.2,
		Messages: []polypus.ChatMessage{
			{Role: polypus.RoleSystem, Content: intentSystem},
			{Role: polypus.RoleUser, Content: user},
		},
	})
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "polypus chat")
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", sirerr.New(sirerr.CodeFailed, op, "empty intent reply")
	}
	return out, nil
}

func formatIntentUser(month, selected string) string {
	month = strings.TrimSpace(month)
	if month == "" {
		month = "(none)"
	}
	return "This is your inbox.\nMonth tab: " + month + "\n\nSelected:\n" + selected
}

func clipSelected(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= intentMaxSelected {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:intentMaxSelected]))
}
