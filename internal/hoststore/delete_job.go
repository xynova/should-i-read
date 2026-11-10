package hoststore

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// DeleteJobConfirmToken is the required confirm string for trash-at-source apply.
const DeleteJobConfirmToken = "trash-at-source"

// Delete job status values.
const (
	DeleteJobStatusPlanned   = "planned"
	DeleteJobStatusApplying  = "applying"
	DeleteJobStatusDone      = "done"
	DeleteJobStatusPartial   = "partial"
	DeleteJobStatusCancelled = "cancelled"
)

// Delete job item status values.
const (
	DeleteJobItemPlanned    = "planned"
	DeleteJobItemUnresolved = "unresolved"
	DeleteJobItemRefused    = "refused"
	DeleteJobItemTrashed    = "trashed"
	DeleteJobItemFailed     = "failed"
)

// DeleteJob is one deletion job header.
type DeleteJob struct {
	ID           string
	Status       string
	ConfirmToken string
	CreatedAt    string
	ConfirmedAt  string
	Error        string
	Items        []DeleteJobItem
}

// DeleteJobItem is one message in a deletion job.
type DeleteJobItem struct {
	JobID      string
	ObjectHash string
	GroupID    string
	GroupLabel string
	Sender     string
	Subject    string
	Collection string
	Seq        int64
	LinkID     string
	EnvelopeID string
	Folder     string
	Status     string
	Error      string
}

// DeleteJobBinding updates resolve fields on a job item.
type DeleteJobBinding struct {
	ObjectHash string
	Collection string
	Seq        int64
	LinkID     string
	EnvelopeID string
	Folder     string
	Status     string
	Error      string
}

// CreateDeleteJobFromCart snapshots current cart rows into a new job.
func (s *Store) CreateDeleteJobFromCart(ctx context.Context, createdAt time.Time) (string, error) {
	const op = "hoststore.Store.CreateDeleteJobFromCart"
	if err := s.requireStore(ctx, op); err != nil {
		return "", err
	}
	if createdAt.IsZero() {
		return "", sirerr.New(sirerr.CodeInvalid, op, "zero created_at")
	}
	cart, err := s.ListDeleteCart(ctx)
	if err != nil {
		return "", err
	}
	if len(cart) == 0 {
		return "", sirerr.New(sirerr.CodeInvalid, op, "empty deletion cart")
	}
	if len(cart) > deleteCartMaxItems {
		return "", sirerr.New(sirerr.CodeInvalid, op, "too many cart items for job").
			With("count", strconv.Itoa(len(cart)))
	}
	jobID, err := newDeleteCartGroupID()
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "new job id")
	}
	stamp := createdAt.UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO delete_jobs (id, status, confirm_token, created_at)
VALUES (?, ?, ?, ?)`,
		jobID, DeleteJobStatusPlanned, DeleteJobConfirmToken, stamp); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "insert job")
	}
	for _, row := range cart {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO delete_job_items (
  job_id, object_hash, group_id, group_label, sender, subject,
  collection, seq, link_id, envelope_id, folder, status
) VALUES (?, ?, ?, ?, ?, ?, '', 0, '', '', '', ?)`,
			jobID, row.ObjectHash, row.GroupID, row.GroupLabel, row.Sender, row.Subject,
			DeleteJobItemPlanned); err != nil {
			return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "insert item").
				With("object_hash", row.ObjectHash)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "commit")
	}
	return jobID, nil
}

// GetDeleteJob loads a job and all items.
func (s *Store) GetDeleteJob(ctx context.Context, jobID string) (DeleteJob, error) {
	const op = "hoststore.Store.GetDeleteJob"
	if err := s.requireStore(ctx, op); err != nil {
		return DeleteJob{}, err
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return DeleteJob{}, sirerr.New(sirerr.CodeInvalid, op, "empty job id")
	}
	var job DeleteJob
	row := s.db.QueryRowContext(ctx, `
SELECT id, status, confirm_token, created_at, confirmed_at, error
FROM delete_jobs WHERE id=?`, id)
	if err := row.Scan(&job.ID, &job.Status, &job.ConfirmToken, &job.CreatedAt, &job.ConfirmedAt, &job.Error); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeleteJob{}, sirerr.New(sirerr.CodeNotFound, op, "job not found").With("job_id", id)
		}
		return DeleteJob{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan job")
	}
	items, err := s.listDeleteJobItems(ctx, id)
	if err != nil {
		return DeleteJob{}, err
	}
	job.Items = items
	return job, nil
}

// LatestDeleteJob returns the most recently created job, if any.
func (s *Store) LatestDeleteJob(ctx context.Context) (DeleteJob, error) {
	const op = "hoststore.Store.LatestDeleteJob"
	if err := s.requireStore(ctx, op); err != nil {
		return DeleteJob{}, err
	}
	var id string
	err := s.db.QueryRowContext(ctx, `
SELECT id FROM delete_jobs ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeleteJob{}, sirerr.New(sirerr.CodeNotFound, op, "no jobs")
		}
		return DeleteJob{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query latest")
	}
	return s.GetDeleteJob(ctx, id)
}

func (s *Store) listDeleteJobItems(ctx context.Context, jobID string) ([]DeleteJobItem, error) {
	const op = "hoststore.Store.listDeleteJobItems"
	rows, err := s.db.QueryContext(ctx, `
SELECT job_id, object_hash, group_id, group_label, sender, subject,
  collection, seq, link_id, envelope_id, folder, status, error
FROM delete_job_items
WHERE job_id=?
ORDER BY object_hash ASC`, jobID)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query items")
	}
	defer func() { _ = rows.Close() }()
	out := []DeleteJobItem{}
	for rows.Next() {
		var it DeleteJobItem
		if err := rows.Scan(
			&it.JobID, &it.ObjectHash, &it.GroupID, &it.GroupLabel, &it.Sender, &it.Subject,
			&it.Collection, &it.Seq, &it.LinkID, &it.EnvelopeID, &it.Folder, &it.Status, &it.Error,
		); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan item")
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate items")
	}
	return out, nil
}

// BindDeleteJobItems updates resolve fields for job items in one transaction.
func (s *Store) BindDeleteJobItems(ctx context.Context, jobID string, bindings []DeleteJobBinding) error {
	const op = "hoststore.Store.BindDeleteJobItems"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty job id")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()
	for _, b := range bindings {
		hash := strings.TrimSpace(b.ObjectHash)
		if hash == "" {
			return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
		}
		status := strings.TrimSpace(b.Status)
		if status == "" {
			status = DeleteJobItemPlanned
		}
		res, err := tx.ExecContext(ctx, `
UPDATE delete_job_items SET
  collection=?, seq=?, link_id=?, envelope_id=?, folder=?, status=?, error=?
WHERE job_id=? AND object_hash=?`,
			b.Collection, b.Seq, b.LinkID, b.EnvelopeID, b.Folder, status, b.Error, id, hash)
		if err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "update item").With("object_hash", hash)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return sirerr.New(sirerr.CodeNotFound, op, "job item not found").
				With("job_id", id).With("object_hash", hash)
		}
	}
	if err := tx.Commit(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "commit")
	}
	return nil
}

// SetDeleteJobStatus updates job header status and optional error text.
func (s *Store) SetDeleteJobStatus(ctx context.Context, jobID, status, errText string, confirmedAt *time.Time) error {
	const op = "hoststore.Store.SetDeleteJobStatus"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty job id")
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty status")
	}
	conf := ""
	if confirmedAt != nil && !confirmedAt.IsZero() {
		conf = confirmedAt.UTC().Format(time.RFC3339)
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE delete_jobs SET status=?, error=?, confirmed_at=CASE WHEN ? != '' THEN ? ELSE confirmed_at END
WHERE id=?`, status, clipRunes(strings.TrimSpace(errText), deleteCartTLDRMax), conf, conf, id)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "update job")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sirerr.New(sirerr.CodeNotFound, op, "job not found").With("job_id", id)
	}
	return nil
}

// SetDeleteJobItemStatus updates one item status.
func (s *Store) SetDeleteJobItemStatus(ctx context.Context, jobID, objectHash, status, errText string) error {
	const op = "hoststore.Store.SetDeleteJobItemStatus"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	id := strings.TrimSpace(jobID)
	hash := strings.TrimSpace(objectHash)
	if id == "" || hash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty job id or object_hash")
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty status")
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE delete_job_items SET status=?, error=?
WHERE job_id=? AND object_hash=?`,
		status, clipRunes(strings.TrimSpace(errText), deleteCartTLDRMax), id, hash)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "update item")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sirerr.New(sirerr.CodeNotFound, op, "job item not found").With("object_hash", hash)
	}
	return nil
}

// ListApplyableDeleteJobItems returns planned and failed items for apply.
func (s *Store) ListApplyableDeleteJobItems(ctx context.Context, jobID string) ([]DeleteJobItem, error) {
	const op = "hoststore.Store.ListApplyableDeleteJobItems"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty job id")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT job_id, object_hash, group_id, group_label, sender, subject,
  collection, seq, link_id, envelope_id, folder, status, error
FROM delete_job_items
WHERE job_id=? AND status IN (?, ?)
ORDER BY object_hash ASC`, id, DeleteJobItemPlanned, DeleteJobItemFailed)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query applyable")
	}
	defer func() { _ = rows.Close() }()
	out := []DeleteJobItem{}
	for rows.Next() {
		var it DeleteJobItem
		if err := rows.Scan(
			&it.JobID, &it.ObjectHash, &it.GroupID, &it.GroupLabel, &it.Sender, &it.Subject,
			&it.Collection, &it.Seq, &it.LinkID, &it.EnvelopeID, &it.Folder, &it.Status, &it.Error,
		); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan item")
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate items")
	}
	return out, nil
}
