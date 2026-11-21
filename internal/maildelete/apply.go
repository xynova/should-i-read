package maildelete

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// TrashDeleter runs one Himalaya message delete.
type TrashDeleter interface {
	MessageDelete(ctx context.Context, folder, envelopeID string) (*pimalaya.Result, error)
}

// deleteReport is the JSON body from himalaya message delete --json.
type deleteReport struct {
	Action string `json:"action"`
	Count  int    `json:"count"`
}

// ParseDeleteReport extracts action from stdout JSON.
func ParseDeleteReport(raw json.RawMessage) (string, int, error) {
	const op = "maildelete.ParseDeleteReport"
	if len(raw) == 0 {
		return "", 0, sirerr.New(sirerr.CodeFailed, op, "empty json")
	}
	var envelope struct {
		Data deleteReport `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Data.Action != "" {
		return envelope.Data.Action, envelope.Data.Count, nil
	}
	var direct deleteReport
	if err := json.Unmarshal(raw, &direct); err != nil {
		return "", 0, sirerr.Wrap(err, sirerr.CodeFailed, op, "parse delete report")
	}
	return direct.Action, direct.Count, nil
}

// TrashSuccess reports whether Himalaya moved the message to trash.
func TrashSuccess(action string, count int) bool {
	return strings.EqualFold(strings.TrimSpace(action), "moved-to-trash") && count >= 1
}

// ApplyAsync starts apply in the background (HTTP 202 path).
func (s *Service) ApplyAsync(jobID, confirm string) error {
	const op = "maildelete.Service.ApplyAsync"
	if s == nil || s.runner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil apply runner")
	}
	return s.runner.ApplyAsync(jobID, confirm)
}

// Apply moves planned items to trash via Himalaya.
func (s *Service) Apply(ctx context.Context, jobID, confirm string) error {
	const op = "maildelete.Service.Apply"
	if s == nil || s.runner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil apply runner")
	}
	if err := s.runner.Apply(ctx, jobID, confirm); err != nil {
		code := sirerr.CodeFailed
		if c, ok := sirerr.AsCode(err); ok {
			code = c
		}
		return sirerr.Wrap(err, code, op, "apply trash job")
	}
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	return s.writeArtifact(job)
}
