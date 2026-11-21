package mailjob

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func (r *Runner) applyItem(ctx context.Context, jobID string, it Item, jobErr *string) (stop bool, err error) {
	folder, env, skip, err := r.resolveItem(ctx, jobID, it)
	if err != nil {
		return true, err
	}
	if skip {
		return false, nil
	}
	return r.hopAndPersist(ctx, jobID, it, folder, env, jobErr)
}

func (r *Runner) hopAndPersist(ctx context.Context, jobID string, it Item, folder, env string, jobErr *string) (stop bool, err error) {
	const op = "mailjob.Runner.hopAndPersist"
	itemCtx, cancel := context.WithTimeout(ctx, r.itemTimeout)
	defer cancel()
	itemCtx, itemSpan := startHopSpan(itemCtx, r.spec, jobID, it.ObjectHash, folder, env)
	hopErr := r.hopper.Hop(itemCtx, folder, env)
	endJobSpan(itemSpan, hopErr)
	if hopErr != nil {
		return r.persistFailure(ctx, jobID, it, hopErr, jobErr)
	}
	if err := r.persistSuccess(ctx, jobID, it); err != nil {
		return true, sirerr.Wrap(err, sirerr.CodeFailed, op, "persist hop success")
	}
	return false, nil
}

func (r *Runner) persistFailure(ctx context.Context, jobID string, it Item, hopErr error, jobErr *string) (stop bool, err error) {
	const op = "mailjob.Runner.persistFailure"
	msg := hopErr.Error()
	if serr := r.store.SetItemStatus(ctx, jobID, it.ObjectHash, r.spec.FailedStatus, msg); serr != nil {
		return true, sirerr.Wrap(serr, sirerr.CodeFailed, op, "set item failed")
	}
	if code, ok := sirerr.AsCode(hopErr); ok && code == sirerr.CodeNeedsReview {
		if jobErr != nil {
			*jobErr = msg
		}
		return true, nil
	}
	return false, nil
}

func (r *Runner) persistSuccess(ctx context.Context, jobID string, it Item) error {
	const op = "mailjob.Runner.persistSuccess"
	if err := r.store.SetItemStatus(ctx, jobID, it.ObjectHash, r.spec.SuccessStatus, ""); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "set item success")
	}
	if err := r.store.UpsertOutcome(ctx, hoststore.MessageOutcome{
		ObjectHash: it.ObjectHash,
		Outcome:    r.spec.Outcome,
		Source:     r.spec.OutcomeSource,
		JobID:      jobID,
		Sender:     strings.TrimSpace(it.Sender),
	}); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert outcome")
	}
	if err := r.store.RemoveCart(ctx, it.ObjectHash); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "remove cart")
	}
	return nil
}
