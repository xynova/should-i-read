package mailjob

import (
	"context"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type trashStore struct {
	inner *hoststore.Store
}

// NewTrashStore adapts the delete job and cart tables to Store.
func NewTrashStore(inner *hoststore.Store) Store {
	return trashStore{inner: inner}
}

func (s trashStore) SetJobStatus(ctx context.Context, jobID, status, errText string, confirmedAt *time.Time) error {
	const op = "mailjob.trashStore.SetJobStatus"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if err := s.inner.SetDeleteJobStatus(ctx, jobID, status, errText, confirmedAt); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "set delete job status")
	}
	return nil
}

func (s trashStore) ListApplyableItems(ctx context.Context, jobID string) ([]Item, error) {
	const op = "mailjob.trashStore.ListApplyableItems"
	if s.inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	rows, err := s.inner.ListApplyableDeleteJobItems(ctx, jobID)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "list applyable delete items")
	}
	out := make([]Item, 0, len(rows))
	for _, it := range rows {
		out = append(out, itemFromDelete(it))
	}
	return out, nil
}

func (s trashStore) BindItems(ctx context.Context, jobID string, bindings []Binding) error {
	const op = "mailjob.trashStore.BindItems"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	in := make([]hoststore.DeleteJobBinding, 0, len(bindings))
	for _, b := range bindings {
		in = append(in, bindingToDelete(b))
	}
	if err := s.inner.BindDeleteJobItems(ctx, jobID, in); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "bind delete job items")
	}
	return nil
}

func (s trashStore) SetItemStatus(ctx context.Context, jobID, objectHash, status, errText string) error {
	const op = "mailjob.trashStore.SetItemStatus"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if err := s.inner.SetDeleteJobItemStatus(ctx, jobID, objectHash, status, errText); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "set delete item status")
	}
	return nil
}

func (s trashStore) GetJob(ctx context.Context, jobID string) (Job, error) {
	const op = "mailjob.trashStore.GetJob"
	if s.inner == nil {
		return Job{}, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	job, err := s.inner.GetDeleteJob(ctx, jobID)
	if err != nil {
		return Job{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "get delete job")
	}
	out := Job{ID: job.ID, Status: job.Status, Error: job.Error}
	for _, it := range job.Items {
		out.Items = append(out.Items, itemFromDelete(it))
	}
	return out, nil
}

func (s trashStore) UpsertOutcome(ctx context.Context, o hoststore.MessageOutcome) error {
	const op = "mailjob.trashStore.UpsertOutcome"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if err := s.inner.UpsertOutcome(ctx, o); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert outcome")
	}
	return nil
}

func (s trashStore) RemoveCart(ctx context.Context, objectHash string) error {
	const op = "mailjob.trashStore.RemoveCart"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if err := s.inner.RemoveDeleteCart(ctx, objectHash); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "remove delete cart")
	}
	return nil
}
