package hoststore

import (
	"math"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	vec := []float64{1, 0, 0}
	blob, dim, err := encodeVector(vec)
	if err != nil || dim != 3 {
		t.Fatalf("encode: dim=%d err=%v", dim, err)
	}
	out, err := decodeVector(blob, dim)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range vec {
		if math.Abs(out[i]-v) > 1e-6 {
			t.Fatalf("round trip %d: %v vs %v", i, out, vec)
		}
	}
}

func TestSimilarRanksNeighbor(t *testing.T) {
	st, ctx := testStore(t)
	model := "test-model"
	q := []float64{1, 0, 0}
	near := []float64{0.99, 0.01, 0}
	far := []float64{0, 1, 0}
	if err := st.PutEmbedding(ctx, "a", model, q); err != nil {
		t.Fatal(err)
	}
	if err := st.PutEmbedding(ctx, "b", model, near); err != nil {
		t.Fatal(err)
	}
	if err := st.PutEmbedding(ctx, "c", model, far); err != nil {
		t.Fatal(err)
	}
	hits, err := st.Similar(ctx, "a", model, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ObjectHash != "b" {
		t.Fatalf("hits %#v", hits)
	}
}

func TestPutEmbeddingEmptyVec(t *testing.T) {
	st, ctx := testStore(t)
	err := st.PutEmbedding(ctx, "x", "m", nil)
	if err == nil {
		t.Fatal("expected invalid for empty vector")
	}
}
