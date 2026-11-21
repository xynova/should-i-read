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

// ReasoningEffortHigh requests high reasoning effort from compatible models.
const ReasoningEffortHigh = "high"

// RoleSystem, RoleUser, and RoleAssistant are OpenAI chat message roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// ChatMessage is one OpenAI chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatReasoning configures model reasoning for a chat completion.
type ChatReasoning struct {
	Effort string `json:"effort,omitempty"`
}

// ChatRequest is a chat completion call.
type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	Temperature float64
	Reasoning   *ChatReasoning
}

type chatCompletionBody struct {
	Model       string         `json:"model"`
	Messages    []ChatMessage  `json:"messages"`
	Temperature float64        `json:"temperature"`
	Reasoning   *ChatReasoning `json:"reasoning,omitempty"`
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
	if err := requireDeadline(ctx, op); err != nil {
		return err
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
func (c *Client) Chat(ctx context.Context, req ChatRequest) (content string, err error) {
	const op = "polypus.Client.Chat"
	if c == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if err = requireDeadline(ctx, op); err != nil {
		return "", err
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "model is empty")
	}
	if len(req.Messages) == 0 {
		return "", sirerr.New(sirerr.CodeInvalid, op, "messages required")
	}
	ctx, span := hopSpan(ctx, "polypus.Client.Chat", model, "/v1/chat/completions")
	defer func() { endHopSpan(span, err) }()
	content, err = runHop(ctx, func() (string, error) {
		out, _, onceErr := c.chatOnce(ctx, model, req)
		return out, onceErr
	})
	return content, err
}

func (c *Client) chatOnce(ctx context.Context, model string, req ChatRequest) (content string, retry bool, err error) {
	const op = "polypus.Client.Chat"
	temp := req.Temperature
	body, err := json.Marshal(chatCompletionBody{
		Model:       model,
		Messages:    req.Messages,
		Temperature: temp,
		Reasoning:   req.Reasoning,
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
