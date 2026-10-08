package polypus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeChat_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	c, err := Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ProbeChat(ctx, "cf_local/gemma"); err != nil {
		t.Fatal(err)
	}
}

func TestProbeChat_noDeadline(t *testing.T) {
	c, err := Create("http://127.0.0.1:1320", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ProbeChat(context.Background(), "m"); err == nil {
		t.Fatal("expected error")
	}
}

func TestChat_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "{\"choice\":\"use:leaf-a\",\"score\":0.9}"}},
			},
		})
	}))
	defer srv.Close()

	c, err := Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Chat(context.Background(), ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("expected content")
	}
}
