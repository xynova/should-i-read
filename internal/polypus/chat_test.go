package polypus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
