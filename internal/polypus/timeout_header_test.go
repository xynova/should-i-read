package polypus

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInstrumentEmbedTimeoutHeader_setsFromDeadline(t *testing.T) {
	t.Parallel()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(TimeoutHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	client := srv.Client()
	InstrumentEmbedTimeoutHeader(client)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got == "" {
		t.Fatal("missing timeout header")
	}
	d, err := time.ParseDuration(got)
	if err != nil {
		t.Fatal(err)
	}
	if d < 10*time.Second || d > 12*time.Second {
		t.Fatalf("header %s", got)
	}
}

func TestInstrumentEmbedTimeoutHeader_skipsChat(t *testing.T) {
	t.Parallel()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(TimeoutHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	client := srv.Client()
	InstrumentEmbedTimeoutHeader(client)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/chat/completions", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got != "" {
		t.Fatalf("chat header %q", got)
	}
}

func TestInstrumentEmbedTimeoutHeader_idempotent(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: http.DefaultTransport}
	InstrumentEmbedTimeoutHeader(client)
	first, ok := client.Transport.(*embedTimeoutTransport)
	if !ok {
		t.Fatalf("%T", client.Transport)
	}
	InstrumentEmbedTimeoutHeader(client)
	second, ok := client.Transport.(*embedTimeoutTransport)
	if !ok || first != second {
		t.Fatal("double wrap")
	}
}

func TestEmbed_sendsTimeoutHeader(t *testing.T) {
	t.Parallel()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(TimeoutHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"index":0,"embedding":[1]}]}`)
	}))
	defer srv.Close()
	client, err := Create(srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if _, err := client.Embed(ctx, EmbedRequest{Model: "m", Input: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("missing timeout header")
	}
}
