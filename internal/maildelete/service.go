package maildelete

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	defaultItemTimeout  = 30 * time.Second
	himalayaMissingHint = "install Himalaya and set pimalaya.himalaya_config via should-i-read configure"
)

// JobArtifact is written to tmp/delete-job-<id>.json.
type JobArtifact struct {
	ID     string            `json:"id"`
	Mode   string            `json:"mode"`
	Status string            `json:"status"`
	Error  string            `json:"error,omitempty"`
	Counts JobCounts         `json:"counts"`
	Items  []JobItemArtifact `json:"items"`
}

// JobCounts tallies item statuses.
type JobCounts struct {
	Planned    int `json:"planned"`
	Unresolved int `json:"unresolved"`
	Refused    int `json:"refused"`
	Trashed    int `json:"trashed"`
	Failed     int `json:"failed"`
}

// JobItemArtifact is one item in the artifact file.
type JobItemArtifact struct {
	ObjectHash string `json:"object_hash"`
	Folder     string `json:"folder,omitempty"`
	EnvelopeID string `json:"envelope_id,omitempty"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// DeleteJobConfig wires the deletion job service.
type DeleteJobConfig struct {
	Store        *hoststore.Store
	Pimdir       PimdirSummaries
	Deleter      TrashDeleter
	Now          func() time.Time
	ItemTimeout  time.Duration
	RepoRoot     string
	ApplyBaseCtx context.Context
}

// Service runs cart deletion jobs.
type Service struct {
	store        *hoststore.Store
	pimdir       PimdirSummaries
	deleter      TrashDeleter
	now          func() time.Time
	itemTimeout  time.Duration
	repoRoot     string
	applyBaseCtx context.Context
	applyMu      sync.Mutex
}

// CreateDeleteJobService builds a Service.
func CreateDeleteJobService(cfg DeleteJobConfig) (*Service, error) {
	const op = "maildelete.CreateDeleteJobService"
	if cfg.Store == nil || cfg.Pimdir == nil || cfg.Now == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing dependencies")
	}
	to := cfg.ItemTimeout
	if to <= 0 {
		to = defaultItemTimeout
	}
	return &Service{
		store:        cfg.Store,
		pimdir:       cfg.Pimdir,
		deleter:      cfg.Deleter,
		now:          cfg.Now,
		itemTimeout:  to,
		repoRoot:     strings.TrimSpace(cfg.RepoRoot),
		applyBaseCtx: cfg.ApplyBaseCtx,
	}, nil
}

// StartJob snapshots the cart, resolves hashes, writes artifact.
func (s *Service) StartJob(ctx context.Context) (art JobArtifact, err error) {
	const op = "maildelete.Service.StartJob"
	ctx, span := startJobSpan(ctx)
	defer func() { endDeleteSpan(span, err) }()
	if s == nil {
		return JobArtifact{}, sirerr.New(sirerr.CodeInvalid, op, "nil service")
	}
	jobID, err := s.store.CreateDeleteJobFromCart(ctx, s.now())
	if err != nil {
		return JobArtifact{}, err
	}
	setDeleteJobID(span, jobID)
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return JobArtifact{}, err
	}
	hashes := make([]string, 0, len(job.Items))
	for _, it := range job.Items {
		hashes = append(hashes, it.ObjectHash)
	}
	bindings, err := ResolveJobBindings(ctx, s.pimdir, hashes)
	if err != nil {
		return JobArtifact{}, err
	}
	if err := s.store.BindDeleteJobItems(ctx, jobID, bindings); err != nil {
		return JobArtifact{}, err
	}
	job, err = s.GetJob(ctx, jobID)
	if err != nil {
		return JobArtifact{}, err
	}
	if err := s.dropRefusedFromCart(ctx, job); err != nil {
		return JobArtifact{}, err
	}
	if err := s.writeArtifact(job); err != nil {
		return JobArtifact{}, err
	}
	return job, nil
}

func (s *Service) dropRefusedFromCart(ctx context.Context, job JobArtifact) error {
	for _, it := range job.Items {
		if it.Status != hoststore.DeleteJobItemRefused {
			continue
		}
		if err := s.store.RemoveDeleteCart(ctx, it.ObjectHash); err != nil {
			return err
		}
	}
	return nil
}

// GetJob loads a job as artifact shape.
func (s *Service) GetJob(ctx context.Context, jobID string) (JobArtifact, error) {
	const op = "maildelete.Service.GetJob"
	job, err := s.store.GetDeleteJob(ctx, jobID)
	if err != nil {
		return JobArtifact{}, err
	}
	job, err = s.recoverStuckApply(ctx, job)
	if err != nil {
		return JobArtifact{}, err
	}
	return jobToArtifact(job), nil
}

// LatestJob loads the newest job.
func (s *Service) LatestJob(ctx context.Context) (JobArtifact, error) {
	job, err := s.store.LatestDeleteJob(ctx)
	if err != nil {
		return JobArtifact{}, err
	}
	job, err = s.recoverStuckApply(ctx, job)
	if err != nil {
		return JobArtifact{}, err
	}
	return jobToArtifact(job), nil
}

func (s *Service) recoverStuckApply(ctx context.Context, job hoststore.DeleteJob) (hoststore.DeleteJob, error) {
	if job.Status != hoststore.DeleteJobStatusApplying || s.deleter != nil {
		return job, nil
	}
	if err := s.markHimalayaMissing(ctx, job.ID); err != nil {
		return job, err
	}
	return s.store.GetDeleteJob(ctx, job.ID)
}

func (s *Service) markHimalayaMissing(ctx context.Context, jobID string) error {
	return s.store.SetDeleteJobStatus(ctx, jobID, hoststore.DeleteJobStatusPlanned, himalayaMissingHint, nil)
}

func himalayaUnavailable(op string) error {
	return sirerr.New(sirerr.CodeUnavailable, op, himalayaMissingHint)
}

func applyMailbox(it hoststore.DeleteJobItem) string {
	if folder := NormalizeFolder(it.Collection); folder != "" {
		return folder
	}
	return NormalizeFolder(it.Folder)
}

// applyEnvelopeID prefers pimdir seq. Himalaya message ids are decimal numbers;
// RFC Message-ID strings in link_id are rejected.
func applyEnvelopeID(it hoststore.DeleteJobItem) string {
	if it.Seq > 0 {
		return strconv.FormatInt(it.Seq, 10)
	}
	if id := numericMessageID(it.EnvelopeID); id != "" {
		return id
	}
	return numericMessageID(it.LinkID)
}

func numericMessageID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return id
}

func (s *Service) refreshApplyBindings(ctx context.Context, jobID string, items []hoststore.DeleteJobItem) error {
	if s == nil || s.pimdir == nil || len(items) == 0 {
		return nil
	}
	hashes := make([]string, 0, len(items))
	for _, it := range items {
		hashes = append(hashes, it.ObjectHash)
	}
	bindings, err := ResolveJobBindings(ctx, s.pimdir, hashes)
	if err != nil {
		return err
	}
	out := make([]hoststore.DeleteJobBinding, 0, len(bindings))
	for _, b := range bindings {
		if b.Status != hoststore.DeleteJobItemPlanned && b.Status != hoststore.DeleteJobItemRefused {
			continue
		}
		if strings.TrimSpace(b.Folder) == "" && b.Status == hoststore.DeleteJobItemPlanned {
			continue
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return nil
	}
	return s.store.BindDeleteJobItems(ctx, jobID, out)
}

// ApplyAsync starts apply in the background (HTTP 202 path).
func (s *Service) ApplyAsync(jobID, confirm string) error {
	const op = "maildelete.Service.ApplyAsync"
	if confirm != hoststore.DeleteJobConfirmToken {
		return sirerr.New(sirerr.CodeInvalid, op, "confirm token mismatch")
	}
	if s.deleter == nil {
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

// Apply moves planned items to trash via Himalaya.
func (s *Service) Apply(ctx context.Context, jobID, confirm string) (err error) {
	const op = "maildelete.Service.Apply"
	ctx, span := startApplySpan(ctx, jobID)
	defer func() { endDeleteSpan(span, err) }()
	if confirm != hoststore.DeleteJobConfirmToken {
		return sirerr.New(sirerr.CodeInvalid, op, "confirm token mismatch")
	}
	if s.deleter == nil {
		_ = s.markHimalayaMissing(ctx, jobID)
		return himalayaUnavailable(op)
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	now := s.now()
	if err := s.store.SetDeleteJobStatus(ctx, jobID, hoststore.DeleteJobStatusApplying, "", &now); err != nil {
		return err
	}
	items, err := s.store.ListApplyableDeleteJobItems(ctx, jobID)
	if err != nil {
		return err
	}
	if err := s.refreshApplyBindings(ctx, jobID, items); err == nil {
		items, err = s.store.ListApplyableDeleteJobItems(ctx, jobID)
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
			bindings, rerr := ResolveJobBindings(ctx, s.pimdir, []string{it.ObjectHash})
			if rerr != nil || len(bindings) == 0 {
				_ = s.store.SetDeleteJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.DeleteJobItemUnresolved, "re-resolve failed")
				continue
			}
			_ = s.store.BindDeleteJobItems(ctx, jobID, bindings)
			folder = bindings[0].Folder
			env = bindings[0].EnvelopeID
			if bindings[0].Status != hoststore.DeleteJobItemPlanned {
				continue
			}
		}
		itemCtx, cancel := context.WithTimeout(ctx, s.itemTimeout)
		itemCtx, itemSpan := startMessageDeleteSpan(itemCtx, jobID, it.ObjectHash, folder, env)
		res, derr := s.deleter.MessageDelete(itemCtx, folder, env)
		cancel()
		if derr != nil {
			endDeleteSpan(itemSpan, derr)
			msg := derr.Error()
			if code, ok := sirerr.AsCode(derr); ok && code == sirerr.CodeNeedsReview {
				_ = s.store.SetDeleteJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.DeleteJobItemFailed, msg)
				jobErr = msg
				break
			}
			_ = s.store.SetDeleteJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.DeleteJobItemFailed, msg)
			continue
		}
		action, count, perr := ParseDeleteReport(res.Raw)
		if perr != nil || !TrashSuccess(action, count) {
			endDeleteSpan(itemSpan, fmt.Errorf("unexpected delete action"))
			_ = s.store.SetDeleteJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.DeleteJobItemFailed, "unexpected delete action")
			continue
		}
		endDeleteSpan(itemSpan, nil)
		_ = s.store.SetDeleteJobItemStatus(ctx, jobID, it.ObjectHash, hoststore.DeleteJobItemTrashed, "")
		if err := s.store.UpsertOutcome(ctx, hoststore.MessageOutcome{
			ObjectHash: it.ObjectHash,
			Outcome:    hoststore.OutcomeTrashedAtSource,
			Source:     hoststore.OutcomeSourceDeleteJob,
			JobID:      jobID,
			Sender:     strings.TrimSpace(it.Sender),
		}); err != nil && jobErr == "" {
			jobErr = err.Error()
		}
		if rerr := s.store.RemoveDeleteCart(ctx, it.ObjectHash); rerr != nil {
			jobErr = rerr.Error()
		}
	}
	job, err := s.store.GetDeleteJob(ctx, jobID)
	if err != nil {
		return err
	}
	status := hoststore.DeleteJobStatusDone
	for _, it := range job.Items {
		if it.Status == hoststore.DeleteJobItemFailed || it.Status == hoststore.DeleteJobItemPlanned {
			status = hoststore.DeleteJobStatusPartial
			break
		}
		if it.Status == hoststore.DeleteJobItemUnresolved {
			status = hoststore.DeleteJobStatusPartial
		}
	}
	if jobErr != "" {
		status = hoststore.DeleteJobStatusPartial
	}
	if err := s.store.SetDeleteJobStatus(ctx, jobID, status, jobErr, &now); err != nil {
		return err
	}
	job, err = s.store.GetDeleteJob(ctx, jobID)
	if err != nil {
		return err
	}
	art := jobToArtifact(job)
	setDeleteCounts(span, art.Counts.Failed, art.Counts.Trashed)
	return s.writeArtifact(art)
}

func (s *Service) writeArtifact(art JobArtifact) error {
	const op = "maildelete.Service.writeArtifact"
	if s.repoRoot == "" {
		return nil
	}
	dir := filepath.Join(s.repoRoot, "tmp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir tmp")
	}
	path := filepath.Join(dir, fmt.Sprintf("delete-job-%s.json", art.ID))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "open artifact")
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(art); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode artifact")
	}
	return nil
}

func jobToArtifact(job hoststore.DeleteJob) JobArtifact {
	art := JobArtifact{
		ID:     job.ID,
		Mode:   "trash_at_source",
		Status: job.Status,
		Error:  job.Error,
	}
	for _, it := range job.Items {
		art.Items = append(art.Items, JobItemArtifact{
			ObjectHash: it.ObjectHash,
			Folder:     it.Folder,
			EnvelopeID: it.EnvelopeID,
			Status:     it.Status,
			Error:      it.Error,
		})
		switch it.Status {
		case hoststore.DeleteJobItemPlanned:
			art.Counts.Planned++
		case hoststore.DeleteJobItemUnresolved:
			art.Counts.Unresolved++
		case hoststore.DeleteJobItemRefused:
			art.Counts.Refused++
		case hoststore.DeleteJobItemTrashed:
			art.Counts.Trashed++
		case hoststore.DeleteJobItemFailed:
			art.Counts.Failed++
		}
	}
	return art
}
