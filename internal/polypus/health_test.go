package polypus_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestCheckOK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"cf_local/test"}]}`))
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
	if got.ModelCount != 1 || got.Status != "ok" {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckZeroModels(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = client.Check(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeUnavailable {
		t.Fatalf("code = %v ok=%v err=%v", code, ok, err)
	}
}

func TestCheckHealthDown(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = client.Check(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
