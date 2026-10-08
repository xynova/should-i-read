package polypus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// EmbedRequest is an OpenAI embeddings call.
type EmbedRequest struct {
	Model string
	Input []string
}

type embedBody struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// ProbeEmbed smokes POST /v1/embeddings for classify readiness.
func (c *Client) ProbeEmbed(ctx context.Context, model string) error {
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, "polypus.Client.ProbeEmbed", "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return sirerr.New(sirerr.CodeInvalid, "polypus.Client.ProbeEmbed", "context must have a deadline")
	}
	_, err := c.Embed(ctx, EmbedRequest{Model: model, Input: []string{"probe"}})
	return err
}

// Embed posts to /v1/embeddings and returns vectors in input order.
func (c *Client) Embed(ctx context.Context, req EmbedRequest) ([][]float64, error) {
	const op = "polypus.Client.Embed"
	if c == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "context must have a deadline")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "model is empty")
	}
	if len(req.Input) == 0 {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "input required")
	}
	const maxAttempts = 2
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, 2*time.Second); err != nil {
				return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "embed retry wait")
			}
		}
		out, retry, err := c.embedOnce(ctx, model, req)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !retry || attempt+1 >= maxAttempts {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) embedOnce(ctx context.Context, model string, req EmbedRequest) ([][]float64, bool, error) {
	const op = "polypus.Client.Embed"
	body, err := json.Marshal(embedBody{Model: model, Input: req.Input})
	if err != nil {
		return nil, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "marshal")
	}
	url := c.BaseURL + "/v1/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := c.ChatHTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, false, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "http")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "read body")
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return nil, true, sirerr.New(sirerr.CodeUnavailable, op, "embeddings unavailable").
			With("status", strconv.Itoa(resp.StatusCode))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, sirerr.New(sirerr.CodeAI, op, "embeddings status").
			With("status", strconv.Itoa(resp.StatusCode)).With("body", truncate(string(raw), 200))
	}
	var parsed embedResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode")
	}
	if len(parsed.Data) != len(req.Input) {
		return nil, false, sirerr.New(sirerr.CodeAI, op, "embedding count mismatch")
	}
	out := make([][]float64, len(req.Input))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(out) {
			return nil, false, sirerr.New(sirerr.CodeAI, op, "embedding index out of range")
		}
		out[item.Index] = item.Embedding
	}
	for i, v := range out {
		if v == nil {
			return nil, false, sirerr.New(sirerr.CodeAI, op, "missing embedding index").With("index", strconv.Itoa(i))
		}
	}
	return out, false, nil
}
