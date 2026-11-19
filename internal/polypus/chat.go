package polypus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	chatRetryMax        = 1
	chatRetryBackoff    = 2 * time.Second
	chatRetryBackoffMax = 2 * time.Second
	chatBreakerFail     = 8
	chatBreakerDelay    = 30 * time.Second
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
func (c *Client) Chat(ctx context.Context, req ChatRequest) (content string, err error) {
	const op = "polypus.Client.Chat"
	if c == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", sirerr.New(sirerr.CodeInvalid, op, "missing deadline")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "model is empty")
	}
	if len(req.Messages) == 0 {
		return "", sirerr.New(sirerr.CodeInvalid, op, "messages required")
	}
	ctx, span := startChatSpan(ctx, model)
	defer func() { endChatSpan(span, err) }()
	retry := defaultChatRetry()
	breaker := defaultChatBreaker()
	content, err = failsafe.With(breaker, retry).WithContext(ctx).Get(func() (string, error) {
		out, _, onceErr := c.chatOnce(ctx, model, req)
		return out, onceErr
	})
	return content, err
}

func startChatSpan(ctx context.Context, model string) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/polypus")
	return tracer.Start(ctx, "polypus.Client.Chat",
		trace.WithAttributes(
			attribute.String("polypus.model", model),
		),
	)
}

func endChatSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func defaultChatRetry() retrypolicy.RetryPolicy[string] {
	return retrypolicy.NewBuilder[string]().
		HandleIf(func(_ string, err error) bool { return isRetryableChatErr(err) }).
		AbortIf(func(_ string, err error) bool {
			return errors.Is(err, context.Canceled) || !isRetryableChatErr(err)
		}).
		WithMaxRetries(chatRetryMax).
		WithBackoff(chatRetryBackoff, chatRetryBackoffMax).
		WithJitterFactor(0.2).
		ReturnLastFailure().
		Build()
}

func defaultChatBreaker() circuitbreaker.CircuitBreaker[string] {
	return circuitbreaker.NewBuilder[string]().
		HandleIf(func(_ string, err error) bool { return isRetryableChatErr(err) }).
		WithFailureThreshold(chatBreakerFail).
		WithDelay(chatBreakerDelay).
		Build()
}

func isRetryableChatErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if code, ok := sirerr.AsCode(err); ok && code == sirerr.CodeInvalid {
		return false
	}
	if code, ok := sirerr.AsCode(err); ok && code == sirerr.CodeUnavailable {
		return true
	}
	lower := strings.ToLower(err.Error())
	for _, n := range []string{"429", "502", "503", "504", "timeout", "connection reset"} {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
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
