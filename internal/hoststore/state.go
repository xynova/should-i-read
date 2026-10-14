package hoststore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// EmbeddingPut is optional data written in the same transaction as Complete.
type EmbeddingPut struct {
	ModelID string
	Vec     []float64
}

// Claim reserves a hash for classification. Skip is true when already complete.
func (s *Store) Claim(ctx context.Context, objectHash string) (token string, skip bool, err error) {
	const op = "hoststore.Store.Claim"
	if err := s.requireStore(ctx, op); err != nil {
		return "", false, err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return "", false, sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	now := s.now.UTC()
	row, found, err := s.loadRow(ctx, hash)
	if err != nil {
		return "", false, err
	}
	if found && isCompleteRow(row) {
		return "", true, nil
	}
	if found && row.Status == "running" && leaseActive(row.LeaseUntil, now) {
		return "", false, sirerr.New(sirerr.CodeInvalid, op, "classification already in progress").With("object_hash", hash)
	}
	tok, err := newClaimToken()
	if err != nil {
		return "", false, err
	}
	lease := now.Add(classifyLease).Format(time.RFC3339)
	stamp := now.Format(time.RFC3339)
	attempt := 1
	if found {
		attempt = row.AttemptCount + 1
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO classify_state (object_hash, status, claim_token, lease_until, attempt_count, updated_at)
VALUES (?, 'running', ?, ?, ?, ?)
ON CONFLICT(object_hash) DO UPDATE SET
  status='running', claim_token=excluded.claim_token, lease_until=excluded.lease_until,
  attempt_count=excluded.attempt_count, updated_at=excluded.updated_at, error=''`,
		hash, tok, lease, attempt, stamp,
	)
	if err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "claim upsert")
	}
	return tok, false, nil
}

// Complete marks a claim successful and optionally stores an embedding in one transaction.
func (s *Store) Complete(ctx context.Context, objectHash, token string, row StateRow, emb *EmbeddingPut) error {
	const op = "hoststore.Store.Complete"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	hash := strings.TrimSpace(objectHash)
	tok := strings.TrimSpace(token)
	if hash == "" || tok == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash or token")
	}
	pathJSON, err := marshalPath(row.Path)
	if err != nil {
		return err
	}
	stamp := s.stamp()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
UPDATE classify_state SET
  status='complete', term_id=?, path_json=?, label=?, source=?, strategy=?,
  kind=?, about=?, shape=?, cosine=?, canonical_term_id=?, reinforced=?,
  judge_score=?, draft_kind=?, catalog_applied=?, sender_term_id=?, sender_label=?,
  sender_maps_to=?, error='', catalog_id=?, classify_model=?, embed_model=?,
  urgency=0, needs_reply_noul=0, involves_decision_noul=0, attention_scored=0, attention_error='',
  claim_token='', lease_until='', updated_at=?
WHERE object_hash=? AND claim_token=? AND status='running'`,
		row.TermID, pathJSON, row.Label, row.Source, row.Strategy,
		row.Kind, row.About, row.Shape, row.Cosine, row.CanonicalTermID, boolInt(row.Reinforced),
		row.JudgeScore, row.DraftKind, boolInt(row.CatalogApplied), row.SenderTermID, row.SenderLabel,
		row.SenderMapsTo, row.CatalogID, row.ClassifyModel, row.EmbedModel, stamp,
		hash, tok,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "complete update")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sirerr.New(sirerr.CodeInvalid, op, "stale or missing claim token").With("object_hash", hash)
	}
	if emb != nil && strings.TrimSpace(emb.ModelID) != "" && len(emb.Vec) > 0 {
		if err := putEmbeddingTx(ctx, tx, hash, emb.ModelID, emb.Vec, stamp); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Fail records a classification error for a claim.
func (s *Store) Fail(ctx context.Context, objectHash, token, errText string) error {
	const op = "hoststore.Store.Fail"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	hash := strings.TrimSpace(objectHash)
	tok := strings.TrimSpace(token)
	if hash == "" || tok == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash or token")
	}
	stamp := s.stamp()
	res, err := s.db.ExecContext(ctx, `
UPDATE classify_state SET status='error', error=?, claim_token='', lease_until='', updated_at=?
WHERE object_hash=? AND claim_token=? AND status='running'`,
		strings.TrimSpace(errText), stamp, hash, tok,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "fail update")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sirerr.New(sirerr.CodeInvalid, op, "stale or missing claim token").With("object_hash", hash)
	}
	return nil
}

// PutAttention stores attention scores on a complete classify row.
func (s *Store) PutAttention(ctx context.Context, objectHash string, urgency, needsReplyNoul, involvesDecisionNoul float64) error {
	const op = "hoststore.Store.PutAttention"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	stamp := s.stamp()
	res, err := s.db.ExecContext(ctx, `
UPDATE classify_state SET
  urgency=?, needs_reply_noul=?, involves_decision_noul=?,
  attention_scored=1, attention_error='', updated_at=?
WHERE object_hash=? AND status='complete'`,
		urgency, needsReplyNoul, involvesDecisionNoul, stamp, hash,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "put attention")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sirerr.New(sirerr.CodeInvalid, op, "row not complete or missing").With("object_hash", hash)
	}
	return nil
}

// Get returns one classify row.
func (s *Store) Get(ctx context.Context, objectHash string) (StateRow, error) {
	const op = "hoststore.Store.Get"
	if err := s.requireStore(ctx, op); err != nil {
		return StateRow{}, err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return StateRow{}, sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	row, found, err := s.loadRow(ctx, hash)
	if err != nil {
		return StateRow{}, err
	}
	if !found {
		return StateRow{}, sirerr.New(sirerr.CodeNotFound, op, "classify row not found").With("object_hash", hash)
	}
	return row, nil
}

// ListCompleteHashes returns object_hashes with complete assignments.
func (s *Store) ListCompleteHashes(ctx context.Context) ([]string, error) {
	const op = "hoststore.Store.ListCompleteHashes"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT object_hash, term_id, path_json, error, status FROM classify_state WHERE status='complete'`)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query complete")
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var hash, termID, pathJSON, errText, status string
		if err := rows.Scan(&hash, &termID, &pathJSON, &errText, &status); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan row")
		}
		path, _ := unmarshalPath(pathJSON)
		row := StateRow{ObjectHash: hash, Status: status, TermID: termID, Path: path, Error: errText}
		if isCompleteRow(row) {
			out = append(out, hash)
		}
	}
	return out, rows.Err()
}

// Counts returns row counts by status.
func (s *Store) Counts(ctx context.Context) (Counts, error) {
	const op = "hoststore.Store.Counts"
	if err := s.requireStore(ctx, op); err != nil {
		return Counts{}, err
	}
	var c Counts
	row := s.db.QueryRowContext(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN status='complete' THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status='error' THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status='running' THEN 1 ELSE 0 END), 0)
FROM classify_state`)
	if err := row.Scan(&c.Complete, &c.Error, &c.Running); err != nil {
		if err == sql.ErrNoRows {
			return Counts{}, nil
		}
		return Counts{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan counts")
	}
	return c, nil
}

// ImportProgress imports legacy JSON progress when the store is empty.
func (s *Store) ImportProgress(ctx context.Context, entries map[string]ProgressEntry) (int, error) {
	const op = "hoststore.Store.ImportProgress"
	if err := s.requireStore(ctx, op); err != nil {
		return 0, err
	}
	c, err := s.Counts(ctx)
	if err != nil {
		return 0, err
	}
	if c.Complete+c.Error+c.Running > 0 {
		return 0, nil
	}
	if len(entries) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()
	n := 0
	stamp := s.stamp()
	for hash, ent := range entries {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		status := ""
		if strings.TrimSpace(ent.Error) != "" {
			status = "error"
		} else if progressComplete(ent) {
			status = "complete"
		} else {
			continue
		}
		pathJSON, err := marshalPath(ent.Path)
		if err != nil {
			return 0, err
		}
		updated := strings.TrimSpace(ent.UpdatedAt)
		if updated == "" {
			updated = stamp
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO classify_state (
  object_hash, status, term_id, path_json, label, source, strategy, kind, about, shape,
  cosine, canonical_term_id, reinforced, judge_score, draft_kind, catalog_applied,
  sender_term_id, sender_label, sender_maps_to, error, updated_at,
  catalog_id, classify_model, embed_model, claim_token, lease_until, attempt_count
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '', '', '', 0)`,
			hash, status, ent.TermID, pathJSON, ent.Label, ent.Source, ent.Strategy, ent.Kind, ent.About, ent.Shape,
			ent.Cosine, ent.CanonicalTermID, boolInt(ent.Reinforced), ent.JudgeScore, ent.DraftKind, boolInt(ent.CatalogApplied),
			ent.SenderTermID, ent.SenderLabel, ent.SenderMapsTo, ent.Error, updated,
		)
		if err != nil {
			return 0, sirerr.Wrap(err, sirerr.CodeFailed, op, "insert import row")
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, sirerr.Wrap(err, sirerr.CodeFailed, op, "commit import")
	}
	return n, nil
}

func (s *Store) loadRow(ctx context.Context, hash string) (StateRow, bool, error) {
	const op = "hoststore.Store.loadRow"
	row := s.db.QueryRowContext(ctx, `
SELECT object_hash, status, term_id, path_json, label, source, strategy, kind, about, shape,
  cosine, canonical_term_id, reinforced, judge_score, draft_kind, catalog_applied,
  sender_term_id, sender_label, sender_maps_to, error, catalog_id, classify_model, embed_model,
  claim_token, lease_until, attempt_count,
  urgency, needs_reply_noul, involves_decision_noul, attention_scored, attention_error,
  updated_at
FROM classify_state WHERE object_hash=?`, hash)
	var r StateRow
	var reinforced, catalogApplied, attentionScored int
	var pathJSON string
	err := row.Scan(
		&r.ObjectHash, &r.Status, &r.TermID, &pathJSON, &r.Label, &r.Source, &r.Strategy,
		&r.Kind, &r.About, &r.Shape, &r.Cosine, &r.CanonicalTermID, &reinforced, &r.JudgeScore,
		&r.DraftKind, &catalogApplied, &r.SenderTermID, &r.SenderLabel, &r.SenderMapsTo,
		&r.Error, &r.CatalogID, &r.ClassifyModel, &r.EmbedModel, &r.ClaimToken, &r.LeaseUntil,
		&r.AttemptCount,
		&r.Urgency, &r.NeedsReplyNoul, &r.InvolvesDecisionNoul, &attentionScored, &r.AttentionError,
		&r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return StateRow{}, false, nil
	}
	if err != nil {
		return StateRow{}, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan row")
	}
	r.Path, _ = unmarshalPath(pathJSON)
	r.Reinforced = reinforced != 0
	r.CatalogApplied = catalogApplied != 0
	r.AttentionScored = attentionScored != 0
	return r, true, nil
}

func isCompleteRow(row StateRow) bool {
	if row.Status != "complete" || strings.TrimSpace(row.Error) != "" {
		return false
	}
	if strings.TrimSpace(row.TermID) != "" {
		return true
	}
	return len(row.Path) > 0
}

func progressComplete(ent ProgressEntry) bool {
	if strings.TrimSpace(ent.Error) != "" {
		return false
	}
	if strings.TrimSpace(ent.TermID) != "" {
		return true
	}
	return len(ent.Path) > 0
}

func leaseActive(leaseUntil string, now time.Time) bool {
	leaseUntil = strings.TrimSpace(leaseUntil)
	if leaseUntil == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, leaseUntil)
	if err != nil {
		return false
	}
	return now.Before(t)
}

func newClaimToken() (string, error) {
	const op = "hoststore.newClaimToken"
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "rand")
	}
	return hex.EncodeToString(b[:]), nil
}

func marshalPath(path []string) (string, error) {
	const op = "hoststore.marshalPath"
	if len(path) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(path)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "marshal path")
	}
	return string(raw), nil
}

func unmarshalPath(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
