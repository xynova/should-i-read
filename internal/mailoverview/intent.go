package mailoverview

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	intentMaxSelected     = 8000
	intentMaxDetailMsgs   = 20
	intentMaxSnippetRunes = 600
	intentSystem          = "This is the operator's own inbox, not a third person's. Speak to the operator as you. The operator selected a region to decide if that mail is noise. Reply in two short parts: a one-sentence TLDR of the selection, then whether it looks safe to mark deletable or why it should stay. Stay with the selected text. Do not claim any mailbox change happened. Do not mention being a model."
	intentDetailSystem    = "This is the operator's own inbox. They already saw a short group summary. Do not list every routine message. First line must start with Routine: and say how many look similar and what they are in one short phrase. Then list at most eight bullets for messages worth reading or clear outliers only (use scores, outlier flag, subject/snippet signals). Each bullet: optional date, subject, one sentence on why it stands out. Skip duplicates of the same failure or release spam. End with one line starting with Verdict: for the whole group (deletable or keep, with brief why). Do not claim any mailbox change happened. Do not mention being a model."
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

// IntentDetailMessage is one row in a sender or kind group.
type IntentDetailMessage struct {
	Date         string
	Sender       string
	Subject      string
	About        string
	Snippet      string
	NeedsReply   float64
	Decision     float64
	Urgency      float64
	OutlierFirst bool
}

// IntentDetailInput expands a bundled selection into per-message TLDRs.
type IntentDetailInput struct {
	Month    string
	Summary  string
	Messages []IntentDetailMessage
}

// Guess returns a short intent reading of the selection.
func (g *IntentGuesser) Guess(ctx context.Context, in IntentInput) (out string, err error) {
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
	ctx, span := startGuessSpan(ctx, op, g.model)
	defer func() { endOverviewSpan(span, err) }()
	user := formatIntentUser(in.Month, selected)
	out, err = g.chat.Chat(ctx, polypus.ChatRequest{
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
		err = sirerr.New(sirerr.CodeFailed, op, "empty intent reply")
		return "", err
	}
	return out, nil
}

// GuessDetail returns per-message TLDR lines for a bundled overview selection.
func (g *IntentGuesser) GuessDetail(ctx context.Context, in IntentDetailInput) (out string, err error) {
	const op = "mailoverview.IntentGuesser.GuessDetail"
	if g == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil guesser")
	}
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", sirerr.New(sirerr.CodeInvalid, op, "context must have a deadline")
	}
	msgs := clipDetailMessages(in.Messages)
	if len(msgs) == 0 {
		return "", sirerr.New(sirerr.CodeInvalid, op, "no messages in selection")
	}
	ctx, span := startGuessSpan(ctx, op, g.model)
	defer func() { endOverviewSpan(span, err) }()
	user := formatIntentDetailUser(in.Month, in.Summary, msgs)
	out, err = g.chat.Chat(ctx, polypus.ChatRequest{
		Model:       g.model,
		Temperature: 0.2,
		Messages: []polypus.ChatMessage{
			{Role: polypus.RoleSystem, Content: intentDetailSystem},
			{Role: polypus.RoleUser, Content: user},
		},
	})
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "polypus chat")
	}
	out = strings.TrimSpace(out)
	if out == "" {
		err = sirerr.New(sirerr.CodeFailed, op, "empty intent detail reply")
		return "", err
	}
	return out, nil
}

func clipDetailMessages(in []IntentDetailMessage) []IntentDetailMessage {
	if len(in) == 0 {
		return nil
	}
	out := make([]IntentDetailMessage, 0, len(in))
	for _, m := range in {
		if strings.TrimSpace(m.Subject) == "" && strings.TrimSpace(m.About) == "" && strings.TrimSpace(m.Snippet) == "" {
			continue
		}
		out = append(out, m)
		if len(out) >= intentMaxDetailMsgs {
			break
		}
	}
	return out
}

func formatIntentUser(month, selected string) string {
	month = strings.TrimSpace(month)
	if month == "" {
		month = "(none)"
	}
	return "This is your inbox.\nMonth tab: " + month + "\n\nSelected:\n" + selected
}

func formatIntentDetailUser(month, summary string, msgs []IntentDetailMessage) string {
	month = strings.TrimSpace(month)
	if month == "" {
		month = "(none)"
	}
	var b strings.Builder
	b.WriteString("This is your inbox.\nMonth tab: ")
	b.WriteString(month)
	b.WriteString("\n\nGroup summary already shown:\n")
	summary = strings.TrimSpace(summary)
	if summary == "" {
		b.WriteString("(none)\n")
	} else {
		b.WriteString(summary)
		b.WriteString("\n")
	}
	b.WriteString("\nMessages (")
	b.WriteString(strconv.Itoa(len(msgs)))
	b.WriteString("):\n")
	for _, m := range msgs {
		b.WriteString(formatDetailLine(m))
	}
	return b.String()
}

func formatDetailLine(m IntentDetailMessage) string {
	var line strings.Builder
	line.WriteString("• ")
	if d := strings.TrimSpace(m.Date); d != "" {
		line.WriteString(d)
		line.WriteString(" · ")
	}
	if s := strings.TrimSpace(m.Subject); s != "" {
		line.WriteString(s)
	} else {
		line.WriteString("(no subject)")
	}
	line.WriteString("\n")
	if a := strings.TrimSpace(m.About); a != "" {
		line.WriteString("  classified: ")
		line.WriteString(a)
		line.WriteString("\n")
	}
	if sn := clipSnippet(m.Snippet); sn != "" {
		line.WriteString("  snippet: ")
		line.WriteString(sn)
		line.WriteString("\n")
	}
	if from := strings.TrimSpace(m.Sender); from != "" {
		line.WriteString("  from: ")
		line.WriteString(from)
		line.WriteString("\n")
	}
	if m.OutlierFirst || m.NeedsReply > 0 || m.Decision > 0 || m.Urgency > 0 {
		line.WriteString("  scores: needs_reply=")
		line.WriteString(strconv.FormatFloat(m.NeedsReply, 'f', 2, 64))
		line.WriteString(" decision=")
		line.WriteString(strconv.FormatFloat(m.Decision, 'f', 2, 64))
		line.WriteString(" urgency=")
		line.WriteString(strconv.FormatFloat(m.Urgency, 'f', 2, 64))
		if m.OutlierFirst {
			line.WriteString(" outlier=yes")
		}
		line.WriteString("\n")
	}
	return line.String()
}

func clipSnippet(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= intentMaxSnippetRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:intentMaxSnippetRunes]))
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
