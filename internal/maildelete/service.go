package maildelete

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailjob"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
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
	store    *hoststore.Store
	pimdir   PimdirSummaries
	deleter  TrashDeleter
	now      func() time.Time
	repoRoot string
	runner   *mailjob.Runner
}

// CreateDeleteJobService builds a Service.
func CreateDeleteJobService(cfg DeleteJobConfig) (*Service, error) {
	const op = "maildelete.CreateDeleteJobService"
	if cfg.Store == nil || cfg.Pimdir == nil || cfg.Now == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing dependencies")
	}
	var hopper mailjob.Hopper
	if cfg.Deleter != nil {
		hopper = trashHopper{deleter: cfg.Deleter}
	}
	runner, err := mailjob.CreateRunner(mailjob.Config{
		Store:        mailjob.NewTrashStore(cfg.Store),
		Resolver:     pimdirResolver{pim: cfg.Pimdir},
		Hopper:       hopper,
		Spec:         mailjob.TrashSpec(),
		Now:          cfg.Now,
		ApplyBaseCtx: cfg.ApplyBaseCtx,
		ItemTimeout:  cfg.ItemTimeout,
	})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeInvalid, op, "create apply runner")
	}
	return &Service{
		store:    cfg.Store,
		pimdir:   cfg.Pimdir,
		deleter:  cfg.Deleter,
		now:      cfg.Now,
		repoRoot: strings.TrimSpace(cfg.RepoRoot),
		runner:   runner,
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
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "create delete job from cart")
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
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "resolve job bindings")
	}
	if err := s.store.BindDeleteJobItems(ctx, jobID, bindings); err != nil {
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "bind delete job items")
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
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "get delete job")
	}
	job, err = s.recoverStuckApply(ctx, job)
	if err != nil {
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "recover stuck apply")
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
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: path is repoRoot/tmp/delete-job-<id>.json.
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
