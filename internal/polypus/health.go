package polypus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// HealthResult is a successful Polypus probe summary.
type HealthResult struct {
	BaseURL    string          `json:"base_url"`
	Status     string          `json:"status"`
	ModelCount int             `json:"model_count"`
	ModelIDs   []string        `json:"model_ids,omitempty"`
	RawHealth  json.RawMessage `json:"raw_health,omitempty"`
}

// Client probes Polypus OpenAI-compatible endpoints.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// Create returns a Client for baseURL (must be non-empty).
func Create(baseURL string, httpClient *http.Client) (*Client, error) {
	const op = "polypus.Create"
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "POLYPUS_BASE_URL is empty")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{BaseURL: baseURL, HTTPClient: httpClient}, nil
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Check hits /health and /v1/models; fails closed when unreachable or zero models.
func (c *Client) Check(ctx context.Context) (*HealthResult, error) {
	const op = "polypus.Client.Check"
	if c == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}

	healthBody, err := c.get(ctx, "/health")
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "polypus /health unreachable").
			With("base_url", c.BaseURL)
	}

	var healthStatus struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(healthBody, &healthStatus)
	status := healthStatus.Status
	if status == "" {
		status = "ok"
	}

	modelsBody, err := c.get(ctx, "/v1/models")
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "polypus /v1/models unreachable").
			With("base_url", c.BaseURL)
	}
	var models modelsResponse
	if err := json.Unmarshal(modelsBody, &models); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode /v1/models")
	}
	ids := make([]string, 0, len(models.Data))
	for _, m := range models.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return nil, sirerr.New(sirerr.CodeUnavailable, op, "no enabled models; check Polypus allow-lists").
			With("base_url", c.BaseURL)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
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
