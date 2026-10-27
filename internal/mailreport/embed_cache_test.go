package mailreport

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/xynova/should-i-read/internal/hoststore"
)

type countingEmbedder struct {
	calls   int
	failN   int
	failErr error
	vecs    [][]float64
	err     error
}

func (c *countingEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	c.calls++
	if c.failN > 0 {
		c.failN--
		if c.failErr != nil {
			return nil, c.failErr
		}
		return nil, errors.New("transient")
	}
	if c.err != nil {
		return nil, c.err
	}
	out := make([][]float64, len(texts))
	for i := range texts {
		if i < len(c.vecs) && c.vecs[i] != nil {
			out[i] = copyVec(c.vecs[i])
		} else {
			out[i] = []float64{float64(i + 1), 0}
		}
	}
	return out, nil
}

func TestCachingEmbedder_secondIdenticalBatchHitsOnce(t *testing.T) {
	inner := &countingEmbedder{}
	cached, err := CreateCachingEmbedder(CachingEmbedderConfig{
		Inner: inner,
		Model: "m1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	texts := []string{"notification > billing > payment"}
	if _, err := cached.Embed(ctx, texts); err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Embed(ctx, texts); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Fatalf("inner calls=%d want 1", inner.calls)
	}
}

func TestCachingEmbedder_batchDedupesRepeatedText(t *testing.T) {
	inner := &countingEmbedder{}
	cached, err := CreateCachingEmbedder(CachingEmbedderConfig{
		Inner: inner,
		Model: "m1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	texts := []string{"a", "a", "b"}
	vecs, err := cached.Embed(ctx, texts)
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Fatalf("inner calls=%d want 1", inner.calls)
	}
	if len(vecs) != 3 || vecs[0][0] != vecs[1][0] {
		t.Fatalf("deduped batch: %+v", vecs)
	}
}

func TestCachingEmbedder_doesNotStoreOnInnerError(t *testing.T) {
	inner := &countingEmbedder{err: errors.New("boom")}
	cached, err := CreateCachingEmbedder(CachingEmbedderConfig{
		Inner: inner,
		Model: "m1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := cached.Embed(ctx, []string{"x"}); err == nil {
		t.Fatal("expected error")
	}
	inner.err = nil
	if _, err := cached.Embed(ctx, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Fatalf("inner calls=%d want 2 (no cache on error)", inner.calls)
	}
}

func TestCachingEmbedder_newTextMisses(t *testing.T) {
	inner := &countingEmbedder{}
	cached, err := CreateCachingEmbedder(CachingEmbedderConfig{
		Inner: inner,
		Model: "m1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := cached.Embed(ctx, []string{"first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Embed(ctx, []string{"second"}); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Fatalf("inner calls=%d want 2", inner.calls)
	}
}

func TestCachingEmbedder_storeHitSkipsInner(t *testing.T) {
	st, ctx := testLedgerCtx(t)
	text := "notification > ci > failure"
	sha := textSHA256(text)
	model := "embed-model"
	vec := []float64{0.3, 0.7}
	if err := st.PutEmbedText(ctx, sha, model, vec); err != nil {
		t.Fatal(err)
	}
	inner := &countingEmbedder{}
	cached, err := CreateCachingEmbedder(CachingEmbedderConfig{
		Inner: inner,
		Model: model,
		Store: st,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := cached.Embed(ctx, []string{text})
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 0 {
		t.Fatalf("inner calls=%d want 0", inner.calls)
	}
	if len(out) != 1 || math.Abs(out[0][0]-vec[0]) > 1e-6 {
		t.Fatalf("out=%v", out)
	}
}

func TestCachingEmbedder_persistAcrossProcess(t *testing.T) {
	st, ctx := testLedgerCtx(t)
	inner := &countingEmbedder{}
	cached1, err := CreateCachingEmbedder(CachingEmbedderConfig{
		Inner: inner,
		Model: "embed-model",
		Store: st,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := "catalog > leaf > breadcrumb"
	if _, err := cached1.Embed(ctx, []string{text}); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Fatalf("first process calls=%d", inner.calls)
	}
	inner2 := &countingEmbedder{}
	cached2, err := CreateCachingEmbedder(CachingEmbedderConfig{
		Inner: inner2,
		Model: "embed-model",
		Store: st,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cached2.Embed(ctx, []string{text}); err != nil {
		t.Fatal(err)
	}
	if inner2.calls != 0 {
		t.Fatalf("second process should hit store, calls=%d", inner2.calls)
	}
}

func testLedgerCtx(t *testing.T) (*hoststore.Store, context.Context) {
	st := testLedger(t)
	ctx := clusterCtx(t)
	return st, ctx
}
