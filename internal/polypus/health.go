package polypus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// HealthResult is a successful Polypus probe summary.
type HealthResult struct {
	BaseURL    string          `json:"base_url"`
	Status     string          `json:"status"`
	ModelCount int             `json:"model_count"`
	ModelIDs   []string        `json:"model_ids,omitempty"`
	RawHealth  json.RawMessage `json:"raw_health,omitempty"`

	ClassifyReady   bool   `json:"classify_ready,omitempty"`
	JudgeModel      string `json:"judge_model,omitempty"`
	JudgeSource     string `json:"judge_source,omitempty"`
	AuthorModel     string `json:"author_model,omitempty"`
	AuthorSource    string `json:"author_source,omitempty"`
	AliasJudgeModel string `json:"alias_judge_model,omitempty"`
}

// Client probes Polypus OpenAI-compatible endpoints.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	ChatHTTP   *http.Client
}

// Create returns a Client for baseURL (must be non-empty).
func Create(baseURL string, httpClient *http.Client) (*Client, error) {
	const op = "polypus.Create"
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "POLYPUS_BASE_URL is empty")
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{BaseURL: baseURL, HTTPClient: httpClient, ChatHTTP: httpClient}, nil
}

// Check hits /health and lists enabled models via GET /v1/apis (merged flavor lists).
func (c *Client) Check(ctx context.Context) (out *HealthResult, err error) {
	const op = "polypus.Client.Check"
	if c == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if err = requireDeadline(ctx, op); err != nil {
		return nil, err
	}
	ctx, span := hopSpan(ctx, "polypus.Client.Check", "", "/health")
	defer func() { endHopSpan(span, err) }()

	healthBody, err := c.get(ctx, "/health")
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "polypus /health unreachable").
			With("base_url", c.BaseURL)
	}

	var healthStatus struct {
		Status string `json:"status"`
	}
	if uerr := json.Unmarshal(healthBody, &healthStatus); uerr != nil {
		return nil, sirerr.Wrap(uerr, sirerr.CodeFailed, op, "decode /health")
	}
	status := healthStatus.Status
	if status == "" {
		status = "ok"
	}

	ids, err := c.listEnabledModelIDs(ctx)
	if err != nil {
		return nil, err
	}

	return &HealthResult{
		BaseURL:    c.BaseURL,
		Status:     status,
		ModelCount: len(ids),
		ModelIDs:   ids,
		RawHealth:  healthBody,
	}, nil
}

func (c *Client) get(ctx context.Context, path string) (json.RawMessage, error) {
	return runHop(ctx, func() (json.RawMessage, error) {
		return c.getOnce(ctx, path)
	})
}

func (c *Client) getOnce(ctx context.Context, path string) (json.RawMessage, error) {
	const op = "polypus.Client.get"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "new request")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeNetwork, op, "http get")
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeNetwork, op, "read body")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, sirerr.New(sirerr.CodeUnavailable, op, "HTTP status").
			With("status", strconv.Itoa(resp.StatusCode)).
			With("body", truncate(string(body), 200))
	}
	return json.RawMessage(body), nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
