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

// AckJobConfirmToken is the required confirm string for seen-at-source apply.
const AckJobConfirmToken = "seen-at-source"

// Ack job item status values. Header status reuses DeleteJobStatus* strings.
const (
	AckJobItemPlanned    = DeleteJobItemPlanned
	AckJobItemUnresolved = DeleteJobItemUnresolved
	AckJobItemRefused    = DeleteJobItemRefused
	AckJobItemSeen       = "seen"
	AckJobItemFailed     = DeleteJobItemFailed
)

// AckJob is one mark-read job header.
type AckJob struct {
	ID           string
	Status       string
	ConfirmToken string
	CreatedAt    string
	ConfirmedAt  string
	Error        string
	Items        []AckJobItem
}

// AckJobItem is one message in a mark-read job.
type AckJobItem struct {
	JobID      string
	ObjectHash string
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

// AckJobBinding updates resolve fields on an ack job item.
type AckJobBinding struct {
	ObjectHash string
	Collection string
	Seq        int64
	LinkID     string
	EnvelopeID string
	Folder     string
	Status     string
	Error      string
}

// CreateAckJobFromCart snapshots current ack cart rows into a new job.
func (s *Store) CreateAckJobFromCart(ctx context.Context, createdAt time.Time) (string, error) {
	const op = "hoststore.Store.CreateAckJobFromCart"
	if err := s.requireStore(ctx, op); err != nil {
		return "", err
	}
	if createdAt.IsZero() {
		return "", sirerr.New(sirerr.CodeInvalid, op, "zero created_at")
	}
	cart, err := s.ListAckCart(ctx)
	if err != nil {
		return "", err
	}
	if len(cart) == 0 {
		return "", sirerr.New(sirerr.CodeInvalid, op, "empty ack cart")
	}
	if len(cart) > deleteCartMaxTotal {
		return "", sirerr.New(sirerr.CodeInvalid, op, "too many ack cart items for job").
			With("count", strconv.Itoa(len(cart))).
			With("max", strconv.Itoa(deleteCartMaxTotal))
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
INSERT INTO ack_jobs (id, status, confirm_token, created_at)
VALUES (?, ?, ?, ?)`,
		jobID, DeleteJobStatusPlanned, AckJobConfirmToken, stamp); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "insert job")
	}
	for _, row := range cart {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ack_job_items (
  job_id, object_hash, sender, subject,
  collection, seq, link_id, envelope_id, folder, status
) VALUES (?, ?, ?, ?, '', 0, '', '', '', ?)`,
			jobID, row.ObjectHash, row.Sender, row.Subject, AckJobItemPlanned); err != nil {
			return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "insert item").
				With("object_hash", row.ObjectHash)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "commit")
	}
	return jobID, nil
}

// GetAckJob loads a job and all items.
func (s *Store) GetAckJob(ctx context.Context, jobID string) (AckJob, error) {
	const op = "hoststore.Store.GetAckJob"
	if err := s.requireStore(ctx, op); err != nil {
		return AckJob{}, err
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return AckJob{}, sirerr.New(sirerr.CodeInvalid, op, "empty job id")
	}
	var job AckJob
	row := s.db.QueryRowContext(ctx, `
SELECT id, status, confirm_token, created_at, confirmed_at, error
FROM ack_jobs WHERE id=?`, id)
	if err := row.Scan(&job.ID, &job.Status, &job.ConfirmToken, &job.CreatedAt, &job.ConfirmedAt, &job.Error); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AckJob{}, sirerr.New(sirerr.CodeNotFound, op, "job not found").With("job_id", id)
		}
		return AckJob{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan job")
	}
	items, err := s.listAckJobItems(ctx, id)
	if err != nil {
		return AckJob{}, err
	}
	job.Items = items
	return job, nil
}

// LatestAckJob returns the most recently created ack job, if any.
func (s *Store) LatestAckJob(ctx context.Context) (AckJob, error) {
	const op = "hoststore.Store.LatestAckJob"
	if err := s.requireStore(ctx, op); err != nil {
		return AckJob{}, err
	}
	var id string
	err := s.db.QueryRowContext(ctx, `
SELECT id FROM ack_jobs ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AckJob{}, sirerr.New(sirerr.CodeNotFound, op, "no jobs")
		}
		return AckJob{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query latest")
	}
	return s.GetAckJob(ctx, id)
}

func (s *Store) listAckJobItems(ctx context.Context, jobID string) ([]AckJobItem, error) {
	const op = "hoststore.Store.listAckJobItems"
	rows, err := s.db.QueryContext(ctx, `
SELECT job_id, object_hash, sender, subject,
  collection, seq, link_id, envelope_id, folder, status, error
FROM ack_job_items
WHERE job_id=?
ORDER BY object_hash ASC`, jobID)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query items")
	}
	defer func() { _ = rows.Close() }()
	out := []AckJobItem{}
	for rows.Next() {
		var it AckJobItem
		if err := rows.Scan(
			&it.JobID, &it.ObjectHash, &it.Sender, &it.Subject,
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

// BindAckJobItems writes resolve fields onto job items.
func (s *Store) BindAckJobItems(ctx context.Context, jobID string, bindings []AckJobBinding) error {
	const op = "hoststore.Store.BindAckJobItems"
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
			status = AckJobItemPlanned
		}
		res, err := tx.ExecContext(ctx, `
UPDATE ack_job_items SET
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

// SetAckJobStatus updates job header status and optional error text.
func (s *Store) SetAckJobStatus(ctx context.Context, jobID, status, errText string, confirmedAt *time.Time) error {
	const op = "hoststore.Store.SetAckJobStatus"
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
UPDATE ack_jobs SET status=?, error=?, confirmed_at=CASE WHEN ? != '' THEN ? ELSE confirmed_at END
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

// SetAckJobItemStatus updates one item status.
func (s *Store) SetAckJobItemStatus(ctx context.Context, jobID, objectHash, status, errText string) error {
	const op = "hoststore.Store.SetAckJobItemStatus"
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
UPDATE ack_job_items SET status=?, error=?
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

// ListApplyableAckJobItems returns planned and failed items for apply.
func (s *Store) ListApplyableAckJobItems(ctx context.Context, jobID string) ([]AckJobItem, error) {
	const op = "hoststore.Store.ListApplyableAckJobItems"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty job id")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT job_id, object_hash, sender, subject,
  collection, seq, link_id, envelope_id, folder, status, error
FROM ack_job_items
WHERE job_id=? AND status IN (?, ?)
ORDER BY object_hash ASC`, id, AckJobItemPlanned, AckJobItemFailed)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query applyable")
	}
	defer func() { _ = rows.Close() }()
	out := []AckJobItem{}
	for rows.Next() {
		var it AckJobItem
		if err := rows.Scan(
			&it.JobID, &it.ObjectHash, &it.Sender, &it.Subject,
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
