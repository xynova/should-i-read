package mailjob

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	defaultItemTimeout = 30 * time.Second
)

// Config wires an apply runner for one Kind.
type Config struct {
	Store        Store
	Resolver     Resolver
	Hopper       Hopper
	Spec         Spec
	Now          func() time.Time
	ApplyBaseCtx context.Context
	ItemTimeout  time.Duration
}

// Runner applies one confirm-gated mailbox job (Seen or Trash).
type Runner struct {
	store        Store
	resolver     Resolver
	hopper       Hopper
	spec         Spec
	now          func() time.Time
	applyBaseCtx context.Context
	itemTimeout  time.Duration
	applyMu      sync.Mutex
}

// CreateRunner builds a Runner. ApplyBaseCtx must be non-nil and have a deadline.
func CreateRunner(cfg Config) (*Runner, error) {
	const op = "mailjob.CreateRunner"
	if cfg.Store == nil || cfg.Resolver == nil || cfg.Now == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing dependencies")
	}
	if cfg.Spec.ConfirmToken == "" || cfg.Spec.Kind == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing job spec")
	}
	if cfg.ApplyBaseCtx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing apply base context")
	}
	if _, ok := cfg.ApplyBaseCtx.Deadline(); !ok {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing apply deadline")
	}
	to := cfg.ItemTimeout
	if to <= 0 {
		to = defaultItemTimeout
	}
	return &Runner{
		store:        cfg.Store,
		resolver:     cfg.Resolver,
		hopper:       cfg.Hopper,
		spec:         cfg.Spec,
		now:          cfg.Now,
		applyBaseCtx: cfg.ApplyBaseCtx,
		itemTimeout:  to,
	}, nil
}

// ApplyAsync starts apply on ApplyBaseCtx (HTTP 202 path). It does not substitute Background.
func (r *Runner) ApplyAsync(jobID, confirm string) error {
	const op = "mailjob.Runner.ApplyAsync"
	if r == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if confirm != r.spec.ConfirmToken {
		return sirerr.New(sirerr.CodeInvalid, op, "confirm token mismatch")
	}
	if r.applyBaseCtx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "missing apply base context")
	}
	if _, ok := r.applyBaseCtx.Deadline(); !ok {
		return sirerr.New(sirerr.CodeInvalid, op, "missing apply deadline")
	}
	if r.hopper == nil {
		if err := r.markHimalayaMissing(r.applyBaseCtx, jobID); err != nil {
			return err
		}
		return himalayaUnavailable(op)
	}
	go func() {
		ctx, span := startApplyAsyncSpan(r.applyBaseCtx, r.spec, jobID)
		defer span.End()
		err := r.Apply(ctx, jobID, confirm)
		if err == nil {
			return
		}
		span.RecordError(err)
		if perr := r.recordAsyncFailure(ctx, jobID, err); perr != nil {
			span.RecordError(perr)
		}
	}()
	return nil
}

// Apply runs resolve, the Himalaya hop, and ledger persist for each planned item.
func (r *Runner) Apply(ctx context.Context, jobID, confirm string) (err error) {
	const op = "mailjob.Runner.Apply"
	if r == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return sirerr.New(sirerr.CodeInvalid, op, "missing deadline")
	}
	ctx, span := startApplySpan(ctx, r.spec, jobID)
	defer func() { endJobSpan(span, err) }()
	if confirm != r.spec.ConfirmToken {
		return sirerr.New(sirerr.CodeInvalid, op, "confirm token mismatch")
	}
	if r.hopper == nil {
		if merr := r.markHimalayaMissing(ctx, jobID); merr != nil {
			return merr
		}
		return himalayaUnavailable(op)
	}
	r.applyMu.Lock()
	defer r.applyMu.Unlock()

	now := r.now()
	if err := r.store.SetJobStatus(ctx, jobID, r.spec.JobApplying, "", &now); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "set job applying")
	}
	items, err := r.store.ListApplyableItems(ctx, jobID)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "list applyable items")
	}
	if err := r.refreshBindings(ctx, jobID, items); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "refresh bindings")
	}
	items, err = r.store.ListApplyableItems(ctx, jobID)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "reload applyable items")
	}
	var jobErr string
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			jobErr = err.Error()
			break
		}
		stop, aerr := r.applyItem(ctx, jobID, it, &jobErr)
		if aerr != nil {
			return aerr
		}
		if stop {
			break
		}
	}
	return r.finalizeJob(ctx, span, jobID, jobErr, now)
}

func (r *Runner) markHimalayaMissing(ctx context.Context, jobID string) error {
	const op = "mailjob.Runner.markHimalayaMissing"
	if err := r.store.SetJobStatus(ctx, jobID, r.spec.JobPlanned, pimalaya.HimalayaMissingHint, nil); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mark himalaya missing")
	}
	return nil
}

func himalayaUnavailable(op string) error {
	return sirerr.New(sirerr.CodeUnavailable, op, pimalaya.HimalayaMissingHint)
}

func (r *Runner) recordAsyncFailure(ctx context.Context, jobID string, applyErr error) error {
	const op = "mailjob.Runner.recordAsyncFailure"
	if applyErr == nil {
		return nil
	}
	job, err := r.store.GetJob(ctx, jobID)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "get job after apply error")
	}
	if job.Status == r.spec.JobDone {
		return nil
	}
	now := r.now()
	if err := r.store.SetJobStatus(ctx, jobID, r.spec.JobPartial, applyErr.Error(), &now); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "set job partial after apply error")
	}
	return nil
}

func (r *Runner) finalizeJob(ctx context.Context, span trace.Span, jobID, jobErr string, now time.Time) error {
	const op = "mailjob.Runner.finalizeJob"
	job, err := r.store.GetJob(ctx, jobID)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "get job after apply")
	}
	status := r.spec.JobDone
	success := 0
	failed := 0
	for _, it := range job.Items {
		switch it.Status {
		case r.spec.FailedStatus, r.spec.PlannedStatus:
			status = r.spec.JobPartial
			if it.Status == r.spec.FailedStatus {
				failed++
			}
		case r.spec.UnresolvedStatus, r.spec.RefusedStatus:
			status = r.spec.JobPartial
		case r.spec.SuccessStatus:
			success++
		}
	}
	if jobErr != "" {
		status = r.spec.JobPartial
	}
	if err := r.store.SetJobStatus(ctx, jobID, status, jobErr, &now); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "set job final status")
	}
	setApplyCounts(span, r.spec, failed, success)
	return nil
}
