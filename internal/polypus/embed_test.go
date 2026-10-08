package polypus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEmbed_noDeadline(t *testing.T) {
	client, err := Create("http://127.0.0.1:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Embed(context.Background(), EmbedRequest{Model: "m", Input: []string{"a"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEmbed_retries429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": []float64{1}}},
		})
	}))
	defer srv.Close()
	client, err := Create(srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	vecs, err := client.Embed(ctx, EmbedRequest{Model: "m", Input: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(vecs) != 1 {
		t.Fatalf("calls=%d vecs=%v", calls, vecs)
	}
}

func TestEmbed_orderByIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 1, "embedding": []float64{0, 1}},
				{"index": 0, "embedding": []float64{1, 0}},
			},
		})
	}))
	defer srv.Close()
	client, err := Create(srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	vecs, err := client.Embed(ctx, EmbedRequest{Model: "m", Input: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || vecs[0][0] != 1 || vecs[1][1] != 1 {
		t.Fatalf("%v", vecs)
	}
}
