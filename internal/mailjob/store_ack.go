package mailjob

import (
	"context"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type ackStore struct {
	inner *hoststore.Store
}

// NewAckStore adapts the ack job and cart tables to Store.
func NewAckStore(inner *hoststore.Store) Store {
	return ackStore{inner: inner}
}

func (s ackStore) SetJobStatus(ctx context.Context, jobID, status, errText string, confirmedAt *time.Time) error {
	const op = "mailjob.ackStore.SetJobStatus"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if err := s.inner.SetAckJobStatus(ctx, jobID, status, errText, confirmedAt); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "set ack job status")
	}
	return nil
}

func (s ackStore) ListApplyableItems(ctx context.Context, jobID string) ([]Item, error) {
	const op = "mailjob.ackStore.ListApplyableItems"
	if s.inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	rows, err := s.inner.ListApplyableAckJobItems(ctx, jobID)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "list applyable ack items")
	}
	out := make([]Item, 0, len(rows))
	for _, it := range rows {
		out = append(out, itemFromAck(it))
	}
	return out, nil
}

func (s ackStore) BindItems(ctx context.Context, jobID string, bindings []Binding) error {
	const op = "mailjob.ackStore.BindItems"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	in := make([]hoststore.AckJobBinding, 0, len(bindings))
	for _, b := range bindings {
		in = append(in, bindingToAck(b))
	}
	if err := s.inner.BindAckJobItems(ctx, jobID, in); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "bind ack job items")
	}
	return nil
}

func (s ackStore) SetItemStatus(ctx context.Context, jobID, objectHash, status, errText string) error {
	const op = "mailjob.ackStore.SetItemStatus"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if err := s.inner.SetAckJobItemStatus(ctx, jobID, objectHash, status, errText); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "set ack item status")
	}
	return nil
}

func (s ackStore) GetJob(ctx context.Context, jobID string) (Job, error) {
	const op = "mailjob.ackStore.GetJob"
	if s.inner == nil {
		return Job{}, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	job, err := s.inner.GetAckJob(ctx, jobID)
	if err != nil {
		return Job{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "get ack job")
	}
	out := Job{ID: job.ID, Status: job.Status, Error: job.Error}
	for _, it := range job.Items {
		out.Items = append(out.Items, itemFromAck(it))
	}
	return out, nil
}

func (s ackStore) UpsertOutcome(ctx context.Context, o hoststore.MessageOutcome) error {
	const op = "mailjob.ackStore.UpsertOutcome"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if err := s.inner.UpsertOutcome(ctx, o); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert outcome")
	}
	return nil
}

func (s ackStore) RemoveCart(ctx context.Context, objectHash string) error {
	const op = "mailjob.ackStore.RemoveCart"
	if s.inner == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if err := s.inner.RemoveAckCart(ctx, objectHash); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "remove ack cart")
	}
	return nil
}
