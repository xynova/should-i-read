package hoststore

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	ShapeStatusAccumulating = "accumulating"
	ShapeStatusActive       = "active"
	ShapeStatusDisabled     = "disabled"
)

// MailShapeRow is one mail_shapes registry row.
type MailShapeRow struct {
	Fingerprint     string
	SenderDomain    string
	SubjectTemplate string
	Kind            string
	TermID          string
	CanonicalTermID string
	LastSlot        string
	PromptHash      string
	EssenceModel    string
	SeedObjectHash  string
	HitCount        int
	ConfirmCount    int
	RejectCount     int
	Status          string
	UpdatedAt       string
}

// ShapeHit is input for RecordShapeHit.
type ShapeHit struct {
	Fingerprint     string
	Domain          string
	Template        string
	Kind            string
	TermID          string
	CanonicalTermID string
	LastSlot        string
	PromptHash      string
	EssenceModel    string
	SeedObjectHash  string
	MinRepeats      int
}

// ShapeHitResult is the outcome of RecordShapeHit.
type ShapeHitResult struct {
	Promoted bool
}

func (h ShapeHit) normalized() ShapeHit {
	return ShapeHit{
		Fingerprint:     strings.TrimSpace(h.Fingerprint),
		Domain:          strings.TrimSpace(h.Domain),
		Template:        strings.TrimSpace(h.Template),
		Kind:            strings.TrimSpace(h.Kind),
		TermID:          strings.TrimSpace(h.TermID),
		CanonicalTermID: strings.TrimSpace(h.CanonicalTermID),
		LastSlot:        strings.TrimSpace(h.LastSlot),
		PromptHash:      strings.TrimSpace(h.PromptHash),
		EssenceModel:    strings.TrimSpace(h.EssenceModel),
		SeedObjectHash:  strings.TrimSpace(h.SeedObjectHash),
		MinRepeats:      h.MinRepeats,
	}
}

func (h ShapeHit) validate(op string) error {
	n := h.normalized()
	if n.Fingerprint == "" || n.Domain == "" || n.Kind == "" || n.LastSlot == "" ||
		n.PromptHash == "" || n.SeedObjectHash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "incomplete shape hit")
	}
	return nil
}

// GetShape returns a row when fingerprint exists.
func (s *Store) GetShape(ctx context.Context, fingerprint string) (MailShapeRow, bool, error) {
	const op = "hoststore.Store.GetShape"
	if err := s.requireStore(ctx, op); err != nil {
		return MailShapeRow{}, false, err
	}
	fp := strings.TrimSpace(fingerprint)
	if fp == "" {
		return MailShapeRow{}, false, sirerr.New(sirerr.CodeInvalid, op, "empty fingerprint")
	}
	var row MailShapeRow
	err := s.db.QueryRowContext(ctx, `
SELECT fingerprint, sender_domain, subject_template, kind, term_id, canonical_term_id,
  last_slot, prompt_hash, essence_model, seed_object_hash, hit_count, confirm_count,
  reject_count, status, updated_at
FROM mail_shapes WHERE fingerprint = ?`, fp).Scan(
		&row.Fingerprint, &row.SenderDomain, &row.SubjectTemplate, &row.Kind, &row.TermID,
		&row.CanonicalTermID, &row.LastSlot, &row.PromptHash, &row.EssenceModel,
		&row.SeedObjectHash, &row.HitCount, &row.ConfirmCount, &row.RejectCount,
		&row.Status, &row.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailShapeRow{}, false, nil
	}
	if err != nil {
		return MailShapeRow{}, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "select mail_shapes")
	}
	return row, true, nil
}

// RecordShapeHit upserts shape accumulation and may promote to active.
func (s *Store) RecordShapeHit(ctx context.Context, hit ShapeHit) (ShapeHitResult, error) {
	const op = "hoststore.Store.RecordShapeHit"
	if err := s.requireStore(ctx, op); err != nil {
		return ShapeHitResult{}, err
	}
	n := hit.normalized()
	if err := n.validate(op); err != nil {
		return ShapeHitResult{}, err
	}
	minRepeats := n.MinRepeats
	if minRepeats <= 0 {
		minRepeats = 3
	}
	stamp := s.stamp()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()

	var existing MailShapeRow
	var found bool
	err = tx.QueryRowContext(ctx, `
SELECT fingerprint, kind, prompt_hash, seed_object_hash, hit_count, status
FROM mail_shapes WHERE fingerprint = ?`, n.Fingerprint).Scan(
		&existing.Fingerprint, &existing.Kind, &existing.PromptHash,
		&existing.SeedObjectHash, &existing.HitCount, &existing.Status,
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		found = false
	case err != nil:
		return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "select for update")
	default:
		found = true
	}

	if !found {
		status := ShapeStatusAccumulating
		promoted := false
		if minRepeats <= 1 {
			status = ShapeStatusActive
			promoted = true
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO mail_shapes (
  fingerprint, sender_domain, subject_template, kind, term_id, canonical_term_id,
  last_slot, prompt_hash, essence_model, seed_object_hash, hit_count, confirm_count,
  reject_count, status, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 0, 0, ?, ?)`,
			n.Fingerprint, n.Domain, n.Template, n.Kind, n.TermID, n.CanonicalTermID,
			n.LastSlot, n.PromptHash, n.EssenceModel, n.SeedObjectHash,
			status, stamp,
		)
		if err != nil {
			return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "insert mail_shapes")
		}
		if err := tx.Commit(); err != nil {
			return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "commit insert")
		}
		return ShapeHitResult{Promoted: promoted}, nil
	}

	if existing.PromptHash != n.PromptHash {
		_, err = tx.ExecContext(ctx, `
UPDATE mail_shapes SET
  sender_domain=?, subject_template=?, kind=?, term_id=?, canonical_term_id=?,
  last_slot=?, prompt_hash=?, essence_model=?, seed_object_hash=?,
  hit_count=1, confirm_count=0, reject_count=0, status=?, updated_at=?
WHERE fingerprint=?`,
			n.Domain, n.Template, n.Kind, n.TermID, n.CanonicalTermID,
			n.LastSlot, n.PromptHash, n.EssenceModel, n.SeedObjectHash,
			ShapeStatusAccumulating, stamp, n.Fingerprint,
		)
		if err != nil {
			return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "reset prompt_hash")
		}
		if err := tx.Commit(); err != nil {
			return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "commit reset")
		}
		return ShapeHitResult{Promoted: false}, nil
	}

	if existing.Status == ShapeStatusDisabled {
		if err := tx.Commit(); err != nil {
			return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "commit disabled no-op")
		}
		return ShapeHitResult{Promoted: false}, nil
	}

	if existing.Kind != n.Kind {
		_, err = tx.ExecContext(ctx, `
UPDATE mail_shapes SET status=?, updated_at=? WHERE fingerprint=?`,
			ShapeStatusDisabled, stamp, n.Fingerprint,
		)
		if err != nil {
			return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "disable kind mismatch")
		}
		if err := tx.Commit(); err != nil {
			return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "commit disable")
		}
		return ShapeHitResult{Promoted: false}, nil
	}

	newHit := existing.HitCount + 1
	promoted := false
	newStatus := existing.Status
	if existing.Status == ShapeStatusAccumulating && newHit >= minRepeats {
		newStatus = ShapeStatusActive
		promoted = true
	}
	_, err = tx.ExecContext(ctx, `
UPDATE mail_shapes SET
  term_id=?, canonical_term_id=?, last_slot=?, essence_model=?,
  hit_count=?, status=?, updated_at=?
WHERE fingerprint=?`,
		n.TermID, n.CanonicalTermID, n.LastSlot, n.EssenceModel,
		newHit, newStatus, stamp, n.Fingerprint,
	)
	if err != nil {
		return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "increment hit_count")
	}
	if err := tx.Commit(); err != nil {
		return ShapeHitResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "commit increment")
	}
	return ShapeHitResult{Promoted: promoted}, nil
}

// ConfirmShape increments confirm_count for an active shape.
func (s *Store) ConfirmShape(ctx context.Context, fingerprint string) error {
	const op = "hoststore.Store.ConfirmShape"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	fp := strings.TrimSpace(fingerprint)
	if fp == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty fingerprint")
	}
	stamp := s.stamp()
	res, err := s.db.ExecContext(ctx, `
UPDATE mail_shapes SET confirm_count = confirm_count + 1, updated_at = ?
WHERE fingerprint = ? AND status = ?`, stamp, fp, ShapeStatusActive)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "update confirm_count")
	}
	n, err := res.RowsAffected()
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "rows affected confirm")
	}
	if n == 0 {
		return nil
	}
	return nil
}

// RejectShape increments reject_count and may disable the shape.
func (s *Store) RejectShape(ctx context.Context, fingerprint string) error {
	const op = "hoststore.Store.RejectShape"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	fp := strings.TrimSpace(fingerprint)
	if fp == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty fingerprint")
	}
	stamp := s.stamp()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()
	var rejectCount, confirmCount int
	var status string
	err = tx.QueryRowContext(ctx, `
SELECT reject_count, confirm_count, status FROM mail_shapes WHERE fingerprint = ?`, fp).
		Scan(&rejectCount, &confirmCount, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "select reject")
	}
	rejectCount++
	newStatus := status
	if rejectCount >= 3 && rejectCount > confirmCount {
		newStatus = ShapeStatusDisabled
	}
	_, err = tx.ExecContext(ctx, `
UPDATE mail_shapes SET reject_count=?, status=?, updated_at=? WHERE fingerprint=?`,
		rejectCount, newStatus, stamp, fp,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "update reject_count")
	}
	if err := tx.Commit(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "commit reject")
	}
	return nil
}
