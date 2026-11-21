package mailoverview

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hostevent"
)

func TestEventsHTTPUnavailable(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h")}).Handler())
	t.Cleanup(ts.Close)
	res, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestEventsHTTPWritesConnected(t *testing.T) {
	t.Parallel()
	bus, err := hostevent.CreateBus(hostevent.BusConfig{Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Events: bus}).Handler())
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	sc := bufio.NewScanner(res.Body)
	if !sc.Scan() {
		t.Fatal("missing connected comment")
	}
	if sc.Text() != ": connected" {
		t.Fatalf("got %q", sc.Text())
	}
}

func TestEventsHTTPStreamsPublish(t *testing.T) {
	t.Parallel()
	bus, err := hostevent.CreateBus(hostevent.BusConfig{Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	bus.Publish(context.Background(), hostevent.Event{
		Source: hostevent.SourceSync,
		Kind:   "scan-progressed",
		Line:   "Scanning imap 2/9",
		Done:   2,
		Total:  9,
	})
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Events: bus}).Handler())
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type %q", ct)
	}
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, "Scanning imap 2/9") {
			return
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		t.Fatal(err)
	}
	t.Fatal("missing streamed event")
}
