package mailoverview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type stubPreview struct {
	prev MessagePreview
	err  error
}

func (s stubPreview) MessagePreview(ctx context.Context, hash string) (MessagePreview, error) {
	if ctx == nil {
		return MessagePreview{}, sirerr.New(sirerr.CodeInvalid, "stubPreview.MessagePreview", "nil context")
	}
	if hash != "h1" {
		return MessagePreview{}, sirerr.New(sirerr.CodeNotFound, "stubPreview.MessagePreview", "unknown hash")
	}
	if s.err != nil {
		return MessagePreview{}, s.err
	}
	return s.prev, nil
}

func TestServePreviewReturnsBodyAndKept(t *testing.T) {
	t.Parallel()
	st, ctx := openKeepTestStore(t)
	if err := st.UpsertOutcome(ctx, hoststore.MessageOutcome{
		ObjectHash: "h1", Outcome: hoststore.OutcomeKept, Source: hoststore.OutcomeSourceOverviewKeep,
	}); err != nil {
		t.Fatal(err)
	}
	srv := (&IntentPageServer{
		HTML:     []byte("h"),
		Outcomes: stubOutcomes{st: st},
		Preview: stubPreview{prev: MessagePreview{
			Subject: "Sub", From: "Ann <a@x>", Date: "2026-11-14", Body: "hello body",
		}},
	}).Handler()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/preview?hash=h1", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out previewResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Body != "hello body" || !out.Kept || out.Subject != "Sub" {
		t.Fatalf("%+v", out)
	}
}

func TestServePreviewUnavailableWithoutMail(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h")}).Handler())
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/preview?hash=h1", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
}
