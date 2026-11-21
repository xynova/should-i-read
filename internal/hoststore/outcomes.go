package hoststore

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// Outcome codes stored in message_outcomes.
const (
	OutcomeTrashedAtSource = "trashed_at_source"
	OutcomeAlreadyInTrash  = "already_in_trash"
	OutcomeKept            = "kept"
	OutcomeAcked           = "acked"
)

// Outcome sources.
const (
	OutcomeSourceDeleteJob     = "delete_job"
	OutcomeSourceReplicaFolder = "replica_folder"
	OutcomeSourceOverviewKeep  = "overview_keep"
	OutcomeSourceAckJob        = "ack_job"
)

// MessageOutcome is one operator label for a message.
type MessageOutcome struct {
	ObjectHash string
	Outcome    string
	Source     string
	JobID      string
	Sender     string
	DecidedAt  string
}

// OutcomeCounts tallies message_outcomes rows.
type OutcomeCounts struct {
	TrashedAtSource int
	AlreadyInTrash  int
	Kept            int
	Acked           int
}

// OutcomeWithKind joins an outcome with classify_state.kind when present.
type OutcomeWithKind struct {
	MessageOutcome
	Kind string
}

func isTrashOutcome(code string) bool {
	switch strings.TrimSpace(code) {
	case OutcomeTrashedAtSource, OutcomeAlreadyInTrash:
		return true
	default:
		return false
	}
}

func hidesFromLiveBoard(code string) bool {
	if isTrashOutcome(code) {
		return true
	}
	return strings.TrimSpace(code) == OutcomeAcked
}

func validateOutcomeRecord(o MessageOutcome) error {
	const op = "hoststore.validateOutcomeRecord"
	if strings.TrimSpace(o.ObjectHash) == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	switch strings.TrimSpace(o.Outcome) {
	case OutcomeTrashedAtSource, OutcomeAlreadyInTrash, OutcomeKept, OutcomeAcked:
	default:
		return sirerr.New(sirerr.CodeInvalid, op, "invalid outcome").With("outcome", o.Outcome)
	}
	switch strings.TrimSpace(o.Source) {
	case OutcomeSourceDeleteJob, OutcomeSourceReplicaFolder, OutcomeSourceOverviewKeep, OutcomeSourceAckJob:
	default:
		return sirerr.New(sirerr.CodeInvalid, op, "invalid source").With("source", o.Source)
	}
	return nil
}

// UpsertOutcome records or updates an operator outcome. Trash overwrites keep.
func (s *Store) UpsertOutcome(ctx context.Context, o MessageOutcome) error {
	const op = "hoststore.Store.UpsertOutcome"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	if err := validateOutcomeRecord(o); err != nil {
		return err
	}
	hash := strings.TrimSpace(o.ObjectHash)
	incoming := strings.TrimSpace(o.Outcome)
	if incoming == OutcomeKept {
		var existing string
		err := s.db.QueryRowContext(ctx, `SELECT outcome FROM message_outcomes WHERE object_hash=?`, hash).Scan(&existing)
		if err == nil && hidesFromLiveBoard(existing) {
			return nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "load outcome")
		}
	}
	stamp := s.stamp()
	if strings.TrimSpace(o.DecidedAt) != "" {
		stamp = strings.TrimSpace(o.DecidedAt)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO message_outcomes (object_hash, outcome, source, job_id, sender, decided_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(object_hash) DO UPDATE SET
  outcome=excluded.outcome,
  source=excluded.source,
  job_id=excluded.job_id,
  sender=CASE WHEN excluded.sender != '' THEN excluded.sender ELSE message_outcomes.sender END,
  decided_at=excluded.decided_at
WHERE excluded.outcome IN ('trashed_at_source', 'already_in_trash')
   OR (
     message_outcomes.outcome NOT IN ('trashed_at_source', 'already_in_trash')
     AND NOT (excluded.outcome = 'kept' AND message_outcomes.outcome = 'acked')
   )`,
		hash, incoming, strings.TrimSpace(o.Source), strings.TrimSpace(o.JobID),
		strings.TrimSpace(o.Sender), stamp,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert outcome")
	}
	return nil
}

// GetOutcome returns one outcome row when present.
func (s *Store) GetOutcome(ctx context.Context, objectHash string) (MessageOutcome, bool, error) {
	const op = "hoststore.Store.GetOutcome"
	if err := s.requireStore(ctx, op); err != nil {
		return MessageOutcome{}, false, err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return MessageOutcome{}, false, sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	var o MessageOutcome
	err := s.db.QueryRowContext(ctx, `
SELECT object_hash, outcome, source, job_id, sender, decided_at
FROM message_outcomes WHERE object_hash=?`, hash).
		Scan(&o.ObjectHash, &o.Outcome, &o.Source, &o.JobID, &o.Sender, &o.DecidedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return MessageOutcome{}, false, nil
	}
	if err != nil {
		return MessageOutcome{}, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "select outcome")
	}
	return o, true, nil
}

// ClearKeptOutcome removes a kept label. Trash rows stay.
func (s *Store) ClearKeptOutcome(ctx context.Context, objectHash string) error {
	const op = "hoststore.Store.ClearKeptOutcome"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	_, err := s.db.ExecContext(ctx, `
DELETE FROM message_outcomes WHERE object_hash=? AND outcome=?`, hash, OutcomeKept)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "delete kept")
	}
	return nil
}

const outcomeHashChunk = 400

// GetOutcomes returns outcomes for the given hashes.
func (s *Store) GetOutcomes(ctx context.Context, hashes []string) (map[string]MessageOutcome, error) {
	const op = "hoststore.Store.GetOutcomes"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	clean := dedupeHashes(hashes)
	out := make(map[string]MessageOutcome, len(clean))
	for i := 0; i < len(clean); i += outcomeHashChunk {
		end := i + outcomeHashChunk
		if end > len(clean) {
			end = len(clean)
		}
		chunk := clean[i:end]
		if err := s.scanOutcomes(ctx, op, chunk, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) scanOutcomes(ctx context.Context, op string, hashes []string, out map[string]MessageOutcome) error {
	if len(hashes) == 0 {
		return nil
	}
	args := make([]any, 0, len(hashes))
	for _, h := range hashes {
		args = append(args, h)
	}
	q := `SELECT object_hash, outcome, source, job_id, sender, decided_at FROM message_outcomes WHERE object_hash IN (` +
		strings.TrimRight(strings.Repeat("?,", len(hashes)), ",") + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "query outcomes")
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var o MessageOutcome
		if err := rows.Scan(&o.ObjectHash, &o.Outcome, &o.Source, &o.JobID, &o.Sender, &o.DecidedAt); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "scan outcome")
		}
		out[o.ObjectHash] = o
	}
	return rows.Err()
}

// CountOutcomes returns row counts per outcome code.
func (s *Store) CountOutcomes(ctx context.Context) (OutcomeCounts, error) {
	const op = "hoststore.Store.CountOutcomes"
	if err := s.requireStore(ctx, op); err != nil {
		return OutcomeCounts{}, err
	}
	var c OutcomeCounts
	rows, err := s.db.QueryContext(ctx, `SELECT outcome, COUNT(*) FROM message_outcomes GROUP BY outcome`)
	if err != nil {
		return OutcomeCounts{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "count outcomes")
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return OutcomeCounts{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan count")
		}
		switch strings.TrimSpace(code) {
		case OutcomeTrashedAtSource:
			c.TrashedAtSource = n
		case OutcomeAlreadyInTrash:
			c.AlreadyInTrash = n
		case OutcomeKept:
			c.Kept = n
		case OutcomeAcked:
			c.Acked = n
		}
	}
	return c, rows.Err()
}

// ListOutcomes returns every outcome row.
func (s *Store) ListOutcomes(ctx context.Context) ([]MessageOutcome, error) {
	const op = "hoststore.Store.ListOutcomes"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT object_hash, outcome, source, job_id, sender, decided_at
FROM message_outcomes ORDER BY decided_at DESC, object_hash`)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "list outcomes")
	}
	defer func() { _ = rows.Close() }()
	var list []MessageOutcome
	for rows.Next() {
		var o MessageOutcome
		if err := rows.Scan(&o.ObjectHash, &o.Outcome, &o.Source, &o.JobID, &o.Sender, &o.DecidedAt); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan outcome")
		}
		list = append(list, o)
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate outcomes")
	}
	return list, nil
}

// ListOutcomesWithKind joins classify_state.kind for rate queries.
func (s *Store) ListOutcomesWithKind(ctx context.Context) ([]OutcomeWithKind, error) {
	const op = "hoststore.Store.ListOutcomesWithKind"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT o.object_hash, o.outcome, o.source, o.job_id, o.sender, o.decided_at,
       COALESCE(c.kind, '')
FROM message_outcomes o
LEFT JOIN classify_state c ON c.object_hash = o.object_hash
ORDER BY o.decided_at DESC, o.object_hash`)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "list outcomes with kind")
	}
	defer func() { _ = rows.Close() }()
	var list []OutcomeWithKind
	for rows.Next() {
		var o OutcomeWithKind
		if err := rows.Scan(&o.ObjectHash, &o.Outcome, &o.Source, &o.JobID, &o.Sender, &o.DecidedAt, &o.Kind); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan")
		}
		list = append(list, o)
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate")
	}
	return list, nil
}

func dedupeHashes(hashes []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range hashes {
		h := strings.TrimSpace(raw)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	return out
}
