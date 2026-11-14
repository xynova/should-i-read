package maildelete

import (
	"context"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Checkout snapshots a nonempty cart into a deletion job, clears the cart, then
// applies Trash at source in the background. An empty cart retries leftover
// planned or failed items on the latest job. The job artifact is the order tracker.
func (s *Service) Checkout(ctx context.Context, confirm string) (JobArtifact, error) {
	const op = "maildelete.Service.Checkout"
	if s == nil {
		return JobArtifact{}, sirerr.New(sirerr.CodeInvalid, op, "nil service")
	}
	if confirm != hoststore.DeleteJobConfirmToken {
		return JobArtifact{}, sirerr.New(sirerr.CodeInvalid, op, "confirm token mismatch")
	}
	cart, err := s.store.ListDeleteCart(ctx)
	if err != nil {
		return JobArtifact{}, err
	}
	if len(cart) > 0 {
		return s.checkoutCart(ctx, confirm)
	}
	return s.checkoutLeftover(ctx, confirm)
}

func (s *Service) checkoutCart(ctx context.Context, confirm string) (JobArtifact, error) {
	job, err := s.StartJob(ctx)
	if err != nil {
		return JobArtifact{}, err
	}
	if err := s.store.ClearDeleteCart(ctx); err != nil {
		return job, err
	}
	return s.startApplyIfLeftover(job, confirm)
}

func (s *Service) checkoutLeftover(ctx context.Context, confirm string) (JobArtifact, error) {
	const op = "maildelete.Service.Checkout"
	job, err := s.LatestJob(ctx)
	if err != nil {
		if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeNotFound {
			return JobArtifact{}, sirerr.New(sirerr.CodeInvalid, op, "cart is empty")
		}
		return JobArtifact{}, err
	}
	switch job.Status {
	case hoststore.DeleteJobStatusApplying:
		return job, nil
	case hoststore.DeleteJobStatusDone:
		return job, nil
	case hoststore.DeleteJobStatusPlanned, hoststore.DeleteJobStatusPartial:
		return s.startApplyIfLeftover(job, confirm)
	default:
		return job, sirerr.New(sirerr.CodeInvalid, op, "job cannot checkout in status "+job.Status)
	}
}

func (s *Service) startApplyIfLeftover(job JobArtifact, confirm string) (JobArtifact, error) {
	if applyLeftover(job) == 0 {
		return job, nil
	}
	if err := s.ApplyAsync(job.ID, confirm); err != nil {
		return job, err
	}
	job.Status = hoststore.DeleteJobStatusApplying
	return job, nil
}

func applyLeftover(job JobArtifact) int {
	return job.Counts.Planned + job.Counts.Failed
}
