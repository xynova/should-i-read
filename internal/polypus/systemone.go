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

// Locked Judge question shape: one type=noul question per packed option id.
// Packed choice ids that contain ":" (use:notification) are sent as question
// keys with ":" replaced by "__"; mailreport remaps answers back to choices.
//
// SystemOneQuestion is one TypeSafe/JEV question on POST /v1/systemone.
type SystemOneQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions,omitempty"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

// SystemOneRequest is a Polypus SystemOne (JEV) call.
type SystemOneRequest struct {
	Model     string
	State     string
	Questions map[string]SystemOneQuestion
}

type systemOneBody struct {
	Model     string                       `json:"model"`
	State     string                       `json:"state"`
	Questions map[string]SystemOneQuestion `json:"questions"`
}

// SystemOneAnswer is one answer in a SystemOne response.
type SystemOneAnswer struct {
	Type   string          `json:"type"`
	Noul   float64         `json:"noul,omitempty"`
	Choice json.RawMessage `json:"choice,omitempty"`
}

// SystemOneResponse is the unwrapped TypeSafe result from Polypus.
type SystemOneResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]SystemOneAnswer `json:"answers"`
}

// SystemOne posts to /v1/systemone.
func (c *Client) SystemOne(ctx context.Context, req SystemOneRequest) (SystemOneResponse, error) {
	const op = "polypus.Client.SystemOne"
	if c == nil {
		return SystemOneResponse{}, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if ctx == nil {
		return SystemOneResponse{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return SystemOneResponse{}, sirerr.New(sirerr.CodeInvalid, op, "model is empty")
	}
	if strings.TrimSpace(req.State) == "" {
		return SystemOneResponse{}, sirerr.New(sirerr.CodeInvalid, op, "state is empty")
	}
	if len(req.Questions) == 0 {
		return SystemOneResponse{}, sirerr.New(sirerr.CodeInvalid, op, "questions required")
	}
	body, err := json.Marshal(systemOneBody{
		Model:     model,
		State:     req.State,
		Questions: req.Questions,
	})
	if err != nil {
		return SystemOneResponse{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "encode request")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/systemone", bytes.NewReader(body))
	if err != nil {
		return SystemOneResponse{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "build request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := c.ChatHTTP
	if client == nil {
		client = c.HTTPClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return SystemOneResponse{}, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "systemone request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return SystemOneResponse{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "read response")
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return SystemOneResponse{}, sirerr.New(sirerr.CodeUnavailable, op, "polypus systemone unavailable").
			With("status", resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SystemOneResponse{}, sirerr.New(sirerr.CodeAI, op, "polypus systemone rejected").
			With("status", resp.Status).With("body", truncate(string(raw), 200))
	}
	var parsed SystemOneResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return SystemOneResponse{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode response")
	}
	if len(parsed.Answers) == 0 {
		return SystemOneResponse{}, sirerr.New(sirerr.CodeAI, op, "empty answers")
	}
	return parsed, nil
}

// ChoiceString returns a string choice from a SystemOne answer.
func (a SystemOneAnswer) ChoiceString() string {
	if len(a.Choice) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(a.Choice, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(string(a.Choice))
}
