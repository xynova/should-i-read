package mailreport

import (
	"context"
	"testing"
	"time"
)

func TestEmbedRequestTimeout_scalesAndCaps(t *testing.T) {
	t.Parallel()
	if embedRequestTimeout(1) != embedTimeoutMin {
		t.Fatalf("n=1 %s", embedRequestTimeout(1))
	}
	n32 := embedRequestTimeout(32)
	if n32 < 20*time.Second || n32 > embedTimeoutMax {
		t.Fatalf("n=32 %s", n32)
	}
	if embedRequestTimeout(500) != embedTimeoutMax {
		t.Fatalf("n=500 should cap at %s got %s", embedTimeoutMax, embedRequestTimeout(500))
	}
}

func TestChunkingEmbedder_splitsCalls(t *testing.T) {
	t.Parallel()
	var calls []int
	var sawDeadline bool
	inner := &recordingEmbedder{fn: func(ctx context.Context, texts []string) ([][]float64, error) {
		if _, ok := ctx.Deadline(); ok {
			sawDeadline = true
		}
		calls = append(calls, len(texts))
		out := make([][]float64, len(texts))
		for i := range out {
			out[i] = []float64{1}
		}
		return out, nil
	}}
	emb, err := CreateChunkingEmbedder(ChunkingEmbedderConfig{Inner: inner, ChunkSize: 3})
	if err != nil {
		t.Fatal(err)
	}
	texts := []string{"a", "b", "c", "d", "e"}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("len %d", len(vecs))
	}
	if len(calls) != 2 || calls[0] != 3 || calls[1] != 2 {
		t.Fatalf("calls %v", calls)
	}
	if !sawDeadline {
		t.Fatal("chunk context missing deadline")
	}
}

type recordingEmbedder struct {
	fn func(context.Context, []string) ([][]float64, error)
}

func (r *recordingEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return r.fn(ctx, texts)
}
