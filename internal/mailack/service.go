package mailack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/mailjob"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// JobArtifact is written to tmp/ack-job-<id>.json.
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
	Seen       int `json:"seen"`
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

// JobConfig wires the mark-read job service.
type JobConfig struct {
	Store        *hoststore.Store
	Pimdir       maildelete.PimdirSummaries
	Flagger      FlagAdder
	Now          func() time.Time
	ItemTimeout  time.Duration
	RepoRoot     string
	ApplyBaseCtx context.Context
}

// Service runs Acked mark-read jobs.
type Service struct {
	store    *hoststore.Store
	pimdir   maildelete.PimdirSummaries
	flagger  FlagAdder
	now      func() time.Time
	repoRoot string
	runner   *mailjob.Runner
}

// CreateAckJobService builds a Service.
func CreateAckJobService(cfg JobConfig) (*Service, error) {
	const op = "mailack.CreateAckJobService"
	if cfg.Store == nil || cfg.Pimdir == nil || cfg.Now == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing dependencies")
	}
	var hopper mailjob.Hopper
	if cfg.Flagger != nil {
		hopper = seenHopper{flagger: cfg.Flagger}
	}
	runner, err := mailjob.CreateRunner(mailjob.Config{
		Store:        mailjob.NewAckStore(cfg.Store),
		Resolver:     pimdirResolver{pim: cfg.Pimdir},
		Hopper:       hopper,
		Spec:         mailjob.SeenSpec(),
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
		flagger:  cfg.Flagger,
		now:      cfg.Now,
		repoRoot: strings.TrimSpace(cfg.RepoRoot),
		runner:   runner,
	}, nil
}

// StartJob snapshots the ack cart, resolves hashes, writes artifact.
func (s *Service) StartJob(ctx context.Context) (art JobArtifact, err error) {
	const op = "mailack.Service.StartJob"
	ctx, span := startJobSpan(ctx)
	defer func() { endAckSpan(span, err) }()
	if s == nil {
		return JobArtifact{}, sirerr.New(sirerr.CodeInvalid, op, "nil service")
	}
	jobID, err := s.store.CreateAckJobFromCart(ctx, s.now())
	if err != nil {
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "create ack job from cart")
	}
	setAckJobID(span, jobID)
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return JobArtifact{}, err
	}
	hashes := make([]string, 0, len(job.Items))
	for _, it := range job.Items {
		hashes = append(hashes, it.ObjectHash)
	}
	bindings, err := maildelete.ResolveJobBindings(ctx, s.pimdir, hashes)
	if err != nil {
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "resolve job bindings")
	}
	if err := s.store.BindAckJobItems(ctx, jobID, toAckBindings(bindings)); err != nil {
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "bind ack job items")
	}
	job, err = s.GetJob(ctx, jobID)
	if err != nil {
		return JobArtifact{}, err
	}
	if err := s.writeArtifact(job); err != nil {
		return JobArtifact{}, err
	}
	return job, nil
}

func toAckBindings(in []hoststore.DeleteJobBinding) []hoststore.AckJobBinding {
	out := make([]hoststore.AckJobBinding, 0, len(in))
	for _, b := range in {
		out = append(out, hoststore.AckJobBinding(b))
	}
	return out
}

// GetJob loads a job as artifact shape.
func (s *Service) GetJob(ctx context.Context, jobID string) (JobArtifact, error) {
	const op = "mailack.Service.GetJob"
	job, err := s.store.GetAckJob(ctx, jobID)
	if err != nil {
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "get ack job")
	}
	job, err = s.recoverStuckApply(ctx, job)
	if err != nil {
		return JobArtifact{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "recover stuck apply")
	}
	return jobToArtifact(job), nil
}

// LatestJob loads the newest ack job.
func (s *Service) LatestJob(ctx context.Context) (JobArtifact, error) {
	job, err := s.store.LatestAckJob(ctx)
	if err != nil {
		return JobArtifact{}, err
	}
	job, err = s.recoverStuckApply(ctx, job)
	if err != nil {
		return JobArtifact{}, err
	}
	return jobToArtifact(job), nil
}

func (s *Service) recoverStuckApply(ctx context.Context, job hoststore.AckJob) (hoststore.AckJob, error) {
	if job.Status != hoststore.DeleteJobStatusApplying || s.flagger != nil {
		return job, nil
	}
	if err := s.markHimalayaMissing(ctx, job.ID); err != nil {
		return job, err
	}
	return s.store.GetAckJob(ctx, job.ID)
}

func (s *Service) markHimalayaMissing(ctx context.Context, jobID string) error {
	return s.store.SetAckJobStatus(ctx, jobID, hoststore.DeleteJobStatusPlanned, pimalaya.HimalayaMissingHint, nil)
}

func (s *Service) writeArtifact(art JobArtifact) error {
	const op = "mailack.Service.writeArtifact"
	if s.repoRoot == "" {
		return nil
	}
	dir := filepath.Join(s.repoRoot, "tmp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir tmp")
	}
	path := filepath.Join(dir, fmt.Sprintf("ack-job-%s.json", art.ID))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: path is repoRoot/tmp/ack-job-<id>.json.
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

func jobToArtifact(job hoststore.AckJob) JobArtifact {
	art := JobArtifact{
		ID:     job.ID,
		Mode:   "seen_at_source",
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
		case hoststore.AckJobItemPlanned:
			art.Counts.Planned++
		case hoststore.AckJobItemUnresolved:
			art.Counts.Unresolved++
		case hoststore.AckJobItemRefused:
			art.Counts.Refused++
		case hoststore.AckJobItemSeen:
			art.Counts.Seen++
		case hoststore.AckJobItemFailed:
			art.Counts.Failed++
		}
	}
	return art
}
