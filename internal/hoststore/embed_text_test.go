package hoststore

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"testing"
)

func TestPutGetEmbedText_roundTrip(t *testing.T) {
	st, ctx := testStore(t)
	sha := textSHA("catalog breadcrumb")
	vec := []float64{0.1, 0.9, 0.0}
	model := "embed-m"
	if err := st.PutEmbedText(ctx, sha, model, vec); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetEmbedText(ctx, sha, model)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(vec) {
		t.Fatalf("len %d want %d", len(got), len(vec))
	}
	for i := range vec {
		if math.Abs(got[i]-vec[i]) > 1e-6 {
			t.Fatalf("vec[%d]=%v want %v", i, got[i], vec[i])
		}
	}
}

func TestSimilar_ignoresEmbedTextCache(t *testing.T) {
	st, ctx := testStore(t)
	model := "embed-m"
	textSHA := textSHA("notification > billing")
	catalogVec := []float64{1, 0, 0}
	if err := st.PutEmbedText(ctx, textSHA, model, catalogVec); err != nil {
		t.Fatal(err)
	}
	mailVec := []float64{0.99, 0.01, 0}
	if err := st.PutEmbedding(ctx, "mail-a", model, mailVec); err != nil {
		t.Fatal(err)
	}
	if err := st.PutEmbedding(ctx, "mail-b", model, []float64{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	hits, err := st.Similar(ctx, "mail-a", model, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.ObjectHash == textSHA {
			t.Fatalf("Similar returned text cache key as neighbor: %#v", hits)
		}
	}
}

func textSHA(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
