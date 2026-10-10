package polypus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// RoleSystem and RoleUser are OpenAI chat message roles.
const (
	RoleSystem = "system"
	RoleUser   = "user"
)

// ChatMessage is one OpenAI chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is a chat completion call.
type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	Temperature float64
}

type chatCompletionBody struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// ProbeChat smokes POST /v1/chat/completions for classify readiness (not operator mail).
func (c *Client) ProbeChat(ctx context.Context, model string) error {
	const op = "polypus.Client.ProbeChat"
	if c == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return sirerr.New(sirerr.CodeInvalid, op, "context must have a deadline")
	}
	_, err := c.Chat(ctx, ChatRequest{
		Model:       strings.TrimSpace(model),
		Temperature: 0,
		Messages: []ChatMessage{
			{Role: RoleUser, Content: "Readiness probe. Reply with exactly: ok"},
		},
	})
	return err
}

// Chat posts to /v1/chat/completions and returns assistant content.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (string, error) {
	const op = "polypus.Client.Chat"
	if c == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "model is empty")
	}
	if len(req.Messages) == 0 {
		return "", sirerr.New(sirerr.CodeInvalid, op, "messages required")
	}
	const maxAttempts = 2
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, 2*time.Second); err != nil {
				return "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "chat retry wait")
			}
		}
		content, retry, err := c.chatOnce(ctx, model, req)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !retry || attempt+1 >= maxAttempts {
			return "", err
		}
	}
	return "", lastErr
}

func sleepContext(ctx context.Context, d time.Duration) error {
	const op = "polypus.sleepContext"
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return sirerr.Wrap(ctx.Err(), sirerr.CodeUnavailable, op, "retry wait")
	case <-t.C:
		return nil
	}
}

func (c *Client) chatOnce(ctx context.Context, model string, req ChatRequest) (content string, retry bool, err error) {
	const op = "polypus.Client.Chat"
	temp := req.Temperature
	body, err := json.Marshal(chatCompletionBody{
		Model:       model,
		Messages:    req.Messages,
		Temperature: temp,
	})
	if err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "encode request")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "build request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := c.ChatHTTP
	if client == nil {
		client = c.HTTPClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "chat request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "read response")
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return "", true, sirerr.New(sirerr.CodeUnavailable, op, "polypus chat unavailable").
			With("model", model).With("status", resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, sirerr.New(sirerr.CodeAI, op, "polypus chat rejected").
			With("model", model).With("status", resp.Status).With("body", truncate(string(raw), 200))
	}
	var parsed chatCompletionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode response")
	}
	if len(parsed.Choices) == 0 {
		return "", false, sirerr.New(sirerr.CodeAI, op, "empty choices")
	}
	content = strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", false, sirerr.New(sirerr.CodeAI, op, "empty message content")
	}
	return content, false, nil
}
