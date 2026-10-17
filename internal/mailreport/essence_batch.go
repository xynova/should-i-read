package mailreport

import (
	"context"
	"strings"
	"sync"

	"github.com/behaviorengineering/strop/pkg/integrity"
	"github.com/behaviorengineering/strop/pkg/openaibatch"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	chatRoleKey    = "role"
	chatContentKey = "content"
)

// ChatLineRunner runs essence chat lines (parallel Polypus chat).
type ChatLineRunner interface {
	RunChatLines(ctx context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error)
}

type essenceChatClient interface {
	Chat(ctx context.Context, req polypus.ChatRequest) (string, error)
}

type parallelChatRunner struct {
	client essenceChatClient
	model  string
	limit  int
}

// NewParallelChatRunner runs essence lines as concurrent POST /v1/chat/completions.
func NewParallelChatRunner(client essenceChatClient, model string, limit int) (ChatLineRunner, error) {
	const op = "mailreport.NewParallelChatRunner"
	if client == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil chat client")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "model is empty")
	}
	if limit <= 0 {
		limit = 8
	}
	return &parallelChatRunner{client: client, model: model, limit: limit}, nil
}

func (r *parallelChatRunner) RunChatLines(ctx context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error) {
	const op = "mailreport.parallelChatRunner.RunChatLines"
	if r == nil || r.client == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "context must have a deadline")
	}
	out := make(map[string]openaibatch.LineResult, len(lines))
	if len(lines) == 0 {
		return out, nil
	}
	sem := make(chan struct{}, r.limit)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ln := range lines {
		wg.Add(1)
		go func(ln openaibatch.ChatLine) {
			defer wg.Done()
			id := strings.TrimSpace(ln.CustomID)
			if id == "" {
				return
			}
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				out[id] = openaibatch.LineResult{ErrMessage: ctx.Err().Error()}
				mu.Unlock()
				return
			}
			content, err := r.client.Chat(ctx, chatRequestFromLine(ln, r.model))
			res := openaibatch.LineResult{Content: content}
			if err != nil {
				res.ErrMessage = err.Error()
			}
			mu.Lock()
			out[id] = res
			mu.Unlock()
		}(ln)
	}
	wg.Wait()
	return out, nil
}

func chatRequestFromLine(ln openaibatch.ChatLine, fallbackModel string) polypus.ChatRequest {
	model := strings.TrimSpace(ln.Model)
	if model == "" {
		model = fallbackModel
	}
	msgs := make([]polypus.ChatMessage, 0, len(ln.Messages))
	for _, m := range ln.Messages {
		msgs = append(msgs, polypus.ChatMessage{Role: m[chatRoleKey], Content: m[chatContentKey]})
	}
	return polypus.ChatRequest{Model: model, Temperature: 0, Messages: msgs}
}

func prefetchOneEssence(ctx context.Context, runner ChatLineRunner, ln openaibatch.ChatLine, firstContent string, escalateModel string) (harness.EssenceOut, error) {
	const op = "mailreport.prefetchOneEssence"
	parsed, parseErr := parseEssenceResponse(firstContent)
	if parseErr == nil {
		return parsed, nil
	}
	shell, err := createEssenceIntegrityShellChatLines(runner, ln, escalateModel, essenceTagCatalog)
	if err != nil {
		return harness.EssenceOut{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "essence integrity shell")
	}
	prior := chatLineToIntegrityMessages(ln)
	raw, runErr := shell.Resume(ctx, prior, firstContent, parseErr)
	if runErr != nil {
		cause := integrity.ValidationError(runErr)
		if cause == nil {
			cause = runErr
		}
		return harness.EssenceOut{}, sirerr.Wrap(cause, sirerr.CodeAI, op, "essence prefetch")
	}
	parsed, parseErr = parseEssenceResponse(raw)
	if parseErr != nil {
		return harness.EssenceOut{}, sirerr.Wrap(parseErr, sirerr.CodeAI, op, "essence parse")
	}
	return parsed, nil
}
