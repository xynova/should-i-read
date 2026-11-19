package mailack

import (
	"context"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// ApplyAsync starts apply in the background (HTTP 202 path).
func (s *Service) ApplyAsync(jobID, confirm string) error {
	const op = "mailack.Service.ApplyAsync"
	if s == nil || s.runner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil apply runner")
	}
	return s.runner.ApplyAsync(jobID, confirm)
}

// Apply marks planned items seen via Himalaya. Messages stay in Inbox.
func (s *Service) Apply(ctx context.Context, jobID, confirm string) error {
	const op = "mailack.Service.Apply"
	if s == nil || s.runner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil apply runner")
	}
	if err := s.runner.Apply(ctx, jobID, confirm); err != nil {
		code := sirerr.CodeFailed
		if c, ok := sirerr.AsCode(err); ok {
			code = c
		}
		return sirerr.Wrap(err, code, op, "apply seen job")
	}
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	return s.writeArtifact(job)
}
