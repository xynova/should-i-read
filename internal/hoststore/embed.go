package hoststore

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"sort"
	"strings"

	"github.com/behaviorengineering/strop/pkg/embed"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const maxSimilarK = 50

// PutEmbedding stores a vector for object_hash and model_id.
func (s *Store) PutEmbedding(ctx context.Context, objectHash, modelID string, vec []float64) error {
	const op = "hoststore.Store.PutEmbedding"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	hash := strings.TrimSpace(objectHash)
	model := strings.TrimSpace(modelID)
	if hash == "" || model == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash or model_id")
	}
	blob, dim, err := encodeVector(vec)
	if err != nil {
		return err
	}
	stamp := s.stamp()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO embeddings (object_hash, model_id, dim, vector, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(object_hash, model_id) DO UPDATE SET dim=excluded.dim, vector=excluded.vector, created_at=excluded.created_at`,
		hash, model, dim, blob, stamp,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "put embedding")
	}
	return nil
}

// GetEmbedding returns a stored vector.
func (s *Store) GetEmbedding(ctx context.Context, objectHash, modelID string) ([]float64, error) {
	const op = "hoststore.Store.GetEmbedding"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	hash := strings.TrimSpace(objectHash)
	model := strings.TrimSpace(modelID)
	if hash == "" || model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty object_hash or model_id")
	}
	var dim int
	var blob []byte
	err := s.db.QueryRowContext(ctx, `SELECT dim, vector FROM embeddings WHERE object_hash=? AND model_id=?`, hash, model).Scan(&dim, &blob)
	if err == sql.ErrNoRows {
		return nil, sirerr.New(sirerr.CodeNotFound, op, "embedding not found").With("object_hash", hash)
	}
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query embedding")
	}
	return decodeVector(blob, dim)
}

// Similar returns top-k neighbors by cosine similarity for the same model_id.
func (s *Store) Similar(ctx context.Context, objectHash, modelID string, k int) ([]Neighbor, error) {
	const op = "hoststore.Store.Similar"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	if k <= 0 {
		k = 5
	}
	if k > maxSimilarK {
		k = maxSimilarK
	}
	query, err := s.GetEmbedding(ctx, objectHash, modelID)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(modelID)
	rows, err := s.db.QueryContext(ctx, `SELECT object_hash, dim, vector FROM embeddings WHERE model_id=?`, model)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "list embeddings")
	}
	defer func() { _ = rows.Close() }()
	var hits []Neighbor
	self := strings.TrimSpace(objectHash)
	for rows.Next() {
		var hash string
		var dim int
		var blob []byte
		if err := rows.Scan(&hash, &dim, &blob); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan embedding")
		}
		if hash == self {
			continue
		}
		other, err := decodeVector(blob, dim)
		if err != nil {
			continue
		}
		cos := embed.CosineSimilarity(query, other)
		hits = append(hits, Neighbor{ObjectHash: hash, Cosine: cos})
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate embeddings")
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Cosine > hits[j].Cosine })
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits, nil
}

func putEmbeddingTx(ctx context.Context, tx *sql.Tx, objectHash, modelID string, vec []float64, stamp string) error {
	const op = "hoststore.putEmbeddingTx"
	blob, dim, err := encodeVector(vec)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO embeddings (object_hash, model_id, dim, vector, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(object_hash, model_id) DO UPDATE SET dim=excluded.dim, vector=excluded.vector, created_at=excluded.created_at`,
		objectHash, strings.TrimSpace(modelID), dim, blob, stamp,
	)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "insert embedding")
	}
	return nil
}

func encodeVector(vec []float64) ([]byte, int, error) {
	const op = "hoststore.encodeVector"
	if len(vec) == 0 {
		return nil, 0, sirerr.New(sirerr.CodeInvalid, op, "empty vector")
	}
	dim := len(vec)
	buf := make([]byte, dim*4)
	for i, v := range vec {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, 0, sirerr.New(sirerr.CodeInvalid, op, "non-finite vector value")
		}
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(float32(v)))
	}
	return buf, dim, nil
}

func decodeVector(blob []byte, dim int) ([]float64, error) {
	const op = "hoststore.decodeVector"
	if dim <= 0 {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "invalid dim")
	}
	if len(blob) != dim*4 {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "vector length mismatch")
	}
	out := make([]float64, dim)
	for i := 0; i < dim; i++ {
		u := binary.LittleEndian.Uint32(blob[i*4 : i*4+4])
		out[i] = float64(math.Float32frombits(u))
	}
	return out, nil
}
