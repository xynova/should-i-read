package hoststore

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	SuggestedReview              = "review"
	SuggestedQuarantineCandidate = "quarantine_candidate"
)

// GetClusterByFingerprint returns a cluster and its members by fingerprint.
func (s *Store) GetClusterByFingerprint(ctx context.Context, fingerprint string) (ClusterRecord, error) {
	const op = "hoststore.Store.GetClusterByFingerprint"
	if err := s.requireStore(ctx, op); err != nil {
		return ClusterRecord{}, err
	}
	fp := strings.TrimSpace(fingerprint)
	if fp == "" {
		return ClusterRecord{}, sirerr.New(sirerr.CodeInvalid, op, "empty fingerprint")
	}
	var rec ClusterRecord
	var needsReply, involvesDecision, attentionScored int
	err := s.db.QueryRowContext(ctx, `
SELECT cluster_id, fingerprint, term_id, sender_domain, size, tldr, suggested_action, tldr_model,
  urgency, needs_reply, involves_decision, attention_scored, updated_at
FROM clusters WHERE fingerprint=?`, fp).Scan(
		&rec.ClusterID, &rec.Fingerprint, &rec.TermID, &rec.SenderDomain, &rec.Size,
		&rec.TLDR, &rec.SuggestedAction, &rec.TLDRModel,
		&rec.Urgency, &needsReply, &involvesDecision, &attentionScored, &rec.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ClusterRecord{}, sirerr.New(sirerr.CodeNotFound, op, "cluster not found").With("fingerprint", fp)
	}
	if err != nil {
		return ClusterRecord{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query cluster")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT object_hash FROM cluster_members WHERE cluster_id=? ORDER BY object_hash`, rec.ClusterID)
	if err != nil {
		return ClusterRecord{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query members")
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return ClusterRecord{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan member")
		}
		rec.MemberHashes = append(rec.MemberHashes, hash)
	}
	if err := rows.Err(); err != nil {
		return ClusterRecord{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate members")
	}
	rec.NeedsReply = needsReply != 0
	rec.InvolvesDecision = involvesDecision != 0
	rec.AttentionScored = attentionScored != 0
	return rec, nil
}

// PutCluster upserts a cluster and replaces its members in one transaction.
func (s *Store) PutCluster(ctx context.Context, rec ClusterRecord) error {
	const op = "hoststore.Store.PutCluster"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	id := strings.TrimSpace(rec.ClusterID)
	fp := strings.TrimSpace(rec.Fingerprint)
	if id == "" || fp == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty cluster_id or fingerprint")
	}
	if rec.Size != len(rec.MemberHashes) {
		return sirerr.New(sirerr.CodeInvalid, op, "size does not match member count").
			With("size", strconv.Itoa(rec.Size)).
			With("members", strconv.Itoa(len(rec.MemberHashes)))
	}
	action := strings.TrimSpace(rec.SuggestedAction)
	if action == "" {
		action = SuggestedReview
	}
	if action != SuggestedReview && action != SuggestedQuarantineCandidate {
		return sirerr.New(sirerr.CodeInvalid, op, "invalid suggested_action").With("action", action)
	}
	stamp := s.stamp()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
INSERT INTO clusters (cluster_id, fingerprint, term_id, sender_domain, size, tldr, suggested_action, tldr_model,
  urgency, needs_reply, involves_decision, attention_scored, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(cluster_id) DO UPDATE SET
  fingerprint=excluded.fingerprint,
  term_id=excluded.term_id,
  sender_domain=excluded.sender_domain,
  size=excluded.size,
  tldr=excluded.tldr,
  suggested_action=excluded.suggested_action,
  tldr_model=excluded.tldr_model,
  urgency=excluded.urgency,
  needs_reply=excluded.needs_reply,
  involves_decision=excluded.involves_decision,
  attention_scored=excluded.attention_scored,
  updated_at=excluded.updated_at`,
		id, fp, rec.TermID, rec.SenderDomain, rec.Size, rec.TLDR, action, rec.TLDRModel,
		rec.Urgency, boolInt(rec.NeedsReply), boolInt(rec.InvolvesDecision), boolInt(rec.AttentionScored), stamp,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert cluster")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cluster_members WHERE cluster_id=?`, id); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "delete members")
	}
	for _, hash := range rec.MemberHashes {
		h := strings.TrimSpace(hash)
		if h == "" {
			return sirerr.New(sirerr.CodeInvalid, op, "empty member object_hash")
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO cluster_members (cluster_id, object_hash) VALUES (?, ?)`, id, h); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "insert member").With("object_hash", h)
		}
	}
	return tx.Commit()
}
