package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestShutdownOverviewHTTP_nilServer(t *testing.T) {
	t.Parallel()
	err := shutdownOverviewHTTP(nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected nil server error")
	}
	if code, ok := sirerr.AsCode(err); !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func TestShutdownOverviewHTTP_unblocksOpenSSE(t *testing.T) {
	t.Parallel()
	base, baseCancel := context.WithCancel(context.Background())
	t.Cleanup(baseCancel)

	started := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, ": connected\n\n"); err != nil {
			return
		}
		flusher.Flush()
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := newOverviewHTTPServer(ln.Addr().String(), mux, base)
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- srv.Serve(ln)
	}()

	reqCtx, reqCancel := context.WithCancel(context.Background())
	t.Cleanup(reqCancel)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+ln.Addr().String()+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		res, doErr := http.DefaultClient.Do(req)
		if doErr != nil {
			return
		}
		defer func() { _ = res.Body.Close() }()
		_, _ = io.Copy(io.Discard, res.Body)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not start")
	}

	applyCalled := false
	began := time.Now()
	if err := shutdownOverviewHTTP(srv, func() { applyCalled = true }, baseCancel, listenErr); err != nil {
		t.Fatal(err)
	}
	if !applyCalled {
		t.Fatal("applyCancel was not called")
	}
	if elapsed := time.Since(began); elapsed > 2*time.Second {
		t.Fatalf("shutdown waited %s; open /events should cancel with the serve context", elapsed)
	}
}
