package polypus_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xynova/should-i-read/internal/polypus"
)

func TestCheckMergesAPICatalogFlavors(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/apis":
			_, _ = w.Write([]byte(`{"object":"api_catalog","apis":[{"id":"openai","models":"/v1/apis/openai/models"},{"id":"systemone","models":"/v1/apis/systemone/models"}]}`))
		case "/v1/apis/openai/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"cf_local/chat"}]}`))
		case "/v1/apis/systemone/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"cf_local/typesafe/jev"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := client.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got.ModelCount != 2 {
		t.Fatalf("model count = %d ids=%v", got.ModelCount, got.ModelIDs)
	}
}
