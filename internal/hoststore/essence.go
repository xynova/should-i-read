package hoststore

import (
	"context"
	"database/sql"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// EssenceFingerprint identifies a cached essence parse (model ids and prompt contract).
type EssenceFingerprint struct {
	EssenceModel  string
	EscalateModel string
	PromptHash    string
}

// EssenceRow is a persisted essence checkpoint.
type EssenceRow struct {
	ObjectHash    string
	Kind          string
	About         string
	Shape         string
	Why           string
	EssenceModel  string
	EscalateModel string
	PromptHash    string
	UpdatedAt     string
}

func (fp EssenceFingerprint) normalized() EssenceFingerprint {
	return EssenceFingerprint{
		EssenceModel:  strings.TrimSpace(fp.EssenceModel),
		EscalateModel: strings.TrimSpace(fp.EscalateModel),
		PromptHash:    strings.TrimSpace(fp.PromptHash),
	}
}

func (fp EssenceFingerprint) matches(row EssenceRow) bool {
	n := fp.normalized()
	return n.EssenceModel == strings.TrimSpace(row.EssenceModel) &&
		n.EscalateModel == strings.TrimSpace(row.EscalateModel) &&
		n.PromptHash == strings.TrimSpace(row.PromptHash)
}

// GetEssence returns a checkpoint when object_hash exists and the fingerprint matches.
func (s *Store) GetEssence(ctx context.Context, objectHash string, fp EssenceFingerprint) (EssenceRow, bool, error) {
	const op = "hoststore.Store.GetEssence"
	if err := s.requireStore(ctx, op); err != nil {
		return EssenceRow{}, false, err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return EssenceRow{}, false, sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	n := fp.normalized()
	if n.EssenceModel == "" || n.PromptHash == "" {
		return EssenceRow{}, false, sirerr.New(sirerr.CodeInvalid, op, "incomplete essence fingerprint")
	}
	var row EssenceRow
	err := s.db.QueryRowContext(ctx, `
SELECT object_hash, kind, about, shape, why, essence_model, escalate_model, prompt_hash, updated_at
FROM essence_cache WHERE object_hash = ?`, hash).Scan(
		&row.ObjectHash, &row.Kind, &row.About, &row.Shape, &row.Why,
		&row.EssenceModel, &row.EscalateModel, &row.PromptHash, &row.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return EssenceRow{}, false, nil
	}
	if err != nil {
		return EssenceRow{}, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "select essence_cache")
	}
	if !n.matches(row) {
		return EssenceRow{}, false, nil
	}
	return row, true, nil
}

// PutEssence upserts an essence checkpoint for the given fingerprint.
func (s *Store) PutEssence(ctx context.Context, objectHash string, fp EssenceFingerprint, row EssenceRow) error {
	const op = "hoststore.Store.PutEssence"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	n := fp.normalized()
	if n.EssenceModel == "" || n.PromptHash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "incomplete essence fingerprint")
	}
	stamp := s.stamp()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO essence_cache (
  object_hash, kind, about, shape, why, essence_model, escalate_model, prompt_hash, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(object_hash) DO UPDATE SET
  kind=excluded.kind, about=excluded.about, shape=excluded.shape, why=excluded.why,
  essence_model=excluded.essence_model, escalate_model=excluded.escalate_model,
  prompt_hash=excluded.prompt_hash, updated_at=excluded.updated_at`,
		hash,
		strings.TrimSpace(row.Kind),
		strings.TrimSpace(row.About),
		strings.TrimSpace(row.Shape),
		strings.TrimSpace(row.Why),
		n.EssenceModel,
		n.EscalateModel,
		n.PromptHash,
		stamp,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert essence_cache")
	}
	return nil
}
