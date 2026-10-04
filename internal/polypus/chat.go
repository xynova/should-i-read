package polypus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
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
	temp := req.Temperature
	body, err := json.Marshal(chatCompletionBody{
		Model:       model,
		Messages:    req.Messages,
		Temperature: temp,
	})
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "encode request")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "build request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := c.ChatHTTP
	if client == nil {
		client = c.HTTPClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "chat request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "read response")
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return "", sirerr.New(sirerr.CodeUnavailable, op, "polypus chat unavailable").
			With("status", resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", sirerr.New(sirerr.CodeAI, op, "polypus chat rejected").
			With("status", resp.Status).With("body", truncate(string(raw), 200))
	}
	var parsed chatCompletionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "decode response")
	}
	if len(parsed.Choices) == 0 {
		return "", sirerr.New(sirerr.CodeAI, op, "empty choices")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", sirerr.New(sirerr.CodeAI, op, "empty message content")
	}
	return content, nil
}
