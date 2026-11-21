package polypus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	const op = "polypus.Client.ProbeEmbed"
	if c == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if err := requireDeadline(ctx, op); err != nil {
		return err
	}
	_, err := c.Embed(ctx, EmbedRequest{Model: model, Input: []string{"probe"}})
	return err
}

// Embed posts to /v1/embeddings and returns vectors in input order.
func (c *Client) Embed(ctx context.Context, req EmbedRequest) (out [][]float64, err error) {
	const op = "polypus.Client.Embed"
	if c == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if err = requireDeadline(ctx, op); err != nil {
		return nil, err
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "model is empty")
	}
	if len(req.Input) == 0 {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "input required")
	}
	ctx, span := hopSpan(ctx, "polypus.Client.Embed", model, "/v1/embeddings")
	defer func() { endHopSpan(span, err) }()
	out, err = runHop(ctx, func() ([][]float64, error) {
		vecs, _, onceErr := c.embedOnce(ctx, model, req)
		return vecs, onceErr
	})
	return out, err
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
	applyTimeoutHeader(httpReq)
	client := c.ChatHTTP
	if client == nil {
		client = c.HTTPClient
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
