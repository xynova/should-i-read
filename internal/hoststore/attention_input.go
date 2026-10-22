package hoststore

import (
	"context"
	"database/sql"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// AttentionInputScores holds cached SystemOne nouls for an attention input.
type AttentionInputScores struct {
	Urgency              float64
	NeedsReplyNoul       float64
	InvolvesDecisionNoul float64
}

// GetAttentionInput returns cached scores or nil when absent.
func (s *Store) GetAttentionInput(ctx context.Context, inputHash, judgeModel, promptHash string) (*AttentionInputScores, error) {
	const op = "hoststore.Store.GetAttentionInput"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	inHash := strings.TrimSpace(inputHash)
	model := strings.TrimSpace(judgeModel)
	pHash := strings.TrimSpace(promptHash)
	if inHash == "" || model == "" || pHash == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty cache key field")
	}
	var scores AttentionInputScores
	err := s.db.QueryRowContext(ctx, `
SELECT urgency, needs_reply_noul, involves_decision_noul
FROM attention_input_cache
WHERE input_hash=? AND judge_model=? AND prompt_hash=?`,
		inHash, model, pHash,
	).Scan(&scores.Urgency, &scores.NeedsReplyNoul, &scores.InvolvesDecisionNoul)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query attention input cache")
	}
	return &scores, nil
}

// PutAttentionInput stores nouls for an attention input key.
func (s *Store) PutAttentionInput(ctx context.Context, inputHash, judgeModel, promptHash string, urgency, needsReplyNoul, involvesDecisionNoul float64) error {
	const op = "hoststore.Store.PutAttentionInput"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	inHash := strings.TrimSpace(inputHash)
	model := strings.TrimSpace(judgeModel)
	pHash := strings.TrimSpace(promptHash)
	if inHash == "" || model == "" || pHash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty cache key field")
	}
	stamp := s.stamp()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO attention_input_cache (input_hash, judge_model, prompt_hash, urgency, needs_reply_noul, involves_decision_noul, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(input_hash, judge_model, prompt_hash) DO UPDATE SET
  urgency=excluded.urgency,
  needs_reply_noul=excluded.needs_reply_noul,
  involves_decision_noul=excluded.involves_decision_noul,
  updated_at=excluded.updated_at`,
		inHash, model, pHash, urgency, needsReplyNoul, involvesDecisionNoul, stamp,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "put attention input cache")
	}
	return nil
}
