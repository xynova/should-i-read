package mailack

import (
	"context"
	"fmt"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// ApplyAsync starts apply in the background (HTTP 202 path).
func (s *Service) ApplyAsync(jobID, confirm string) error {
	const op = "mailack.Service.ApplyAsync"
	if confirm != hoststore.AckJobConfirmToken {
		return sirerr.New(sirerr.CodeInvalid, op, "confirm token mismatch")
	}
	if s.flagger == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.markHimalayaMissing(ctx, jobID)
		return himalayaUnavailable(op)
	}
	go func() {
		base := s.applyBaseCtx
		if base == nil {
			base = context.Background()
		}
		ctx, cancel := context.WithTimeout(base, 2*time.Hour)
		defer cancel()
		_ = s.Apply(ctx, jobID, confirm)
	}()
	return nil
}

// Apply marks planned items seen via Himalaya. Messages stay in Inbox.
func (s *Service) Apply(ctx context.Context, jobID, confirm string) (err error) {
	const op = "mailack.Service.Apply"
	ctx, span := startApplySpan(ctx, jobID)
	defer func() { endAckSpan(span, err) }()
	if confirm != hoststore.AckJobConfirmToken {
		return sirerr.New(sirerr.CodeInvalid, op, "confirm token mismatch")
	}
	if s.flagger == nil {
		_ = s.markHimalayaMissing(ctx, jobID)
		return himalayaUnavailable(op)
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	now := s.now()
	if err := s.store.SetAckJobStatus(ctx, jobID, hoststore.DeleteJobStatusApplying, "", &now); err != nil {
		return err
	}
	items, err := s.store.ListApplyableAckJobItems(ctx, jobID)
	if err != nil {
		return err
	}
	if err := s.refreshApplyBindings(ctx, jobID, items); err == nil {
		items, err = s.store.ListApplyableAckJobItems(ctx, jobID)
		if err != nil {
			return err
		}
	}
	var jobErr string
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			jobErr = err.Error()
			break
		}
		folder := applyMailbox(it)
		env := applyEnvelopeID(it)
		if folder == "" || env == "" {
			bindings, rerr := maildelete.ResolveJobBindings(ctx, s.pimdir, []string{it.ObjectHash})
			if rerr != nil || len(bindings) == 0 {
				_ = s.store.SetAckJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.AckJobItemUnresolved, "re-resolve failed")
				continue
			}
			_ = s.store.BindAckJobItems(ctx, jobID, toAckBindings(bindings))
			folder = bindings[0].Folder
			env = bindings[0].EnvelopeID
			if bindings[0].Status != hoststore.AckJobItemPlanned {
				continue
			}
		}
		itemCtx, cancel := context.WithTimeout(ctx, s.itemTimeout)
		itemCtx, itemSpan := startFlagAddSpan(itemCtx, jobID, it.ObjectHash, folder, env)
		res, ferr := s.flagger.FlagAdd(itemCtx, folder, env, pimalaya.FlagSeen)
		cancel()
		if ferr != nil {
			endAckSpan(itemSpan, ferr)
			msg := ferr.Error()
			if code, ok := sirerr.AsCode(ferr); ok && code == sirerr.CodeNeedsReview {
				_ = s.store.SetAckJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.AckJobItemFailed, msg)
				jobErr = msg
				break
			}
			_ = s.store.SetAckJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.AckJobItemFailed, msg)
			continue
		}
		count, flags, perr := ParseFlagAddReport(res.Raw)
		if perr != nil || !FlagAddSuccess(count, flags) {
			endAckSpan(itemSpan, fmt.Errorf("unexpected flag add"))
			_ = s.store.SetAckJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.AckJobItemFailed, "unexpected flag add")
			continue
		}
		endAckSpan(itemSpan, nil)
		_ = s.store.SetAckJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.AckJobItemSeen, "")
		if err := s.store.UpsertOutcome(ctx, hoststore.MessageOutcome{
			ObjectHash: it.ObjectHash,
			Outcome:    hoststore.OutcomeAcked,
			Source:     hoststore.OutcomeSourceAckJob,
			JobID:      jobID,
			Sender:     it.Sender,
		}); err != nil && jobErr == "" {
			jobErr = err.Error()
		}
		if rerr := s.store.RemoveAckCart(ctx, it.ObjectHash); rerr != nil {
			jobErr = rerr.Error()
		}
	}
	job, err := s.store.GetAckJob(ctx, jobID)
	if err != nil {
		return err
	}
	status := hoststore.DeleteJobStatusDone
	for _, it := range job.Items {
		if it.Status == hoststore.AckJobItemFailed || it.Status == hoststore.AckJobItemPlanned {
			status = hoststore.DeleteJobStatusPartial
			break
		}
		if it.Status == hoststore.AckJobItemUnresolved {
			status = hoststore.DeleteJobStatusPartial
		}
	}
	if jobErr != "" {
		status = hoststore.DeleteJobStatusPartial
	}
	if err := s.store.SetAckJobStatus(ctx, jobID, status, jobErr, &now); err != nil {
		return err
	}
	job, err = s.store.GetAckJob(ctx, jobID)
	if err != nil {
		return err
	}
	art := jobToArtifact(job)
	setAckCounts(span, art.Counts.Failed, art.Counts.Seen)
	return s.writeArtifact(art)
}
