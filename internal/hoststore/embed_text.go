package hoststore

import (
	"context"
	"database/sql"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// GetEmbedText returns a cached text embedding or nil when absent.
func (s *Store) GetEmbedText(ctx context.Context, textSHA256, modelID string) ([]float64, error) {
	const op = "hoststore.Store.GetEmbedText"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	sha := strings.TrimSpace(textSHA256)
	model := strings.TrimSpace(modelID)
	if sha == "" || model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty text_sha256 or model_id")
	}
	var dim int
	var blob []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT dim, vector FROM embed_text_cache WHERE text_sha256=? AND model_id=?`,
		sha, model,
	).Scan(&dim, &blob)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query embed text cache")
	}
	return decodeVector(blob, dim)
}

// PutEmbedText stores a text-keyed embedding vector.
func (s *Store) PutEmbedText(ctx context.Context, textSHA256, modelID string, vec []float64) error {
	const op = "hoststore.Store.PutEmbedText"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	sha := strings.TrimSpace(textSHA256)
	model := strings.TrimSpace(modelID)
	if sha == "" || model == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty text_sha256 or model_id")
	}
	blob, dim, err := encodeVector(vec)
	if err != nil {
		return err
	}
	stamp := s.stamp()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO embed_text_cache (text_sha256, model_id, dim, vector, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(text_sha256, model_id) DO UPDATE SET dim=excluded.dim, vector=excluded.vector, created_at=excluded.created_at`,
		sha, model, dim, blob, stamp,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "put embed text cache")
	}
	return nil
}
