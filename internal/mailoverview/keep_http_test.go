package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
)

type stubOutcomes struct {
	st *hoststore.Store
}

func (s stubOutcomes) UpsertOutcome(ctx context.Context, o hoststore.MessageOutcome) error {
	return s.st.UpsertOutcome(ctx, o)
}

func (s stubOutcomes) ListOutcomes(ctx context.Context) ([]hoststore.MessageOutcome, error) {
	return s.st.ListOutcomes(ctx)
}

func (s stubOutcomes) GetOutcome(ctx context.Context, objectHash string) (hoststore.MessageOutcome, bool, error) {
	return s.st.GetOutcome(ctx, objectHash)
}

func (s stubOutcomes) ClearKeptOutcome(ctx context.Context, objectHash string) error {
	return s.st.ClearKeptOutcome(ctx, objectHash)
}

func openKeepTestStore(t *testing.T) (*hoststore.Store, context.Context) {
	t.Helper()
	path := t.TempDir() + "/ledger.db"
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = st.Close() })
	return st, ctx
}

func TestServeKeepUpsertsKept(t *testing.T) {
	t.Parallel()
	st, ctx := openKeepTestStore(t)
	body, err := json.Marshal(keepAddRequest{Items: []keepJSONItem{
		{ObjectHash: "h1", Sender: "me@example.com"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	srv := (&IntentPageServer{HTML: []byte("h"), Outcomes: stubOutcomes{st: st}}).Handler()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/keep", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	got, ok, err := st.GetOutcome(ctx, "h1")
	if err != nil || !ok || got.Outcome != hoststore.OutcomeKept {
		t.Fatalf("outcome %+v ok %v err %v", got, ok, err)
	}
}

func TestServeKeepTrashWins(t *testing.T) {
	t.Parallel()
	st, ctx := openKeepTestStore(t)
	if err := st.UpsertOutcome(ctx, hoststore.MessageOutcome{
		ObjectHash: "h1",
		Outcome:    hoststore.OutcomeTrashedAtSource,
		Source:     hoststore.OutcomeSourceDeleteJob,
	}); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(keepAddRequest{Items: []keepJSONItem{{ObjectHash: "h1"}}})
	srv := (&IntentPageServer{
		HTML:     []byte("h"),
		Outcomes: stubOutcomes{st: st},
		Actioned: stubActioned{m: map[string]maildelete.ActionedInfo{
			"h1": {ObjectHash: "h1", Outcome: maildelete.OutcomeTrashedAtSource},
		}},
	}).Handler()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/keep", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	got, _, _ := st.GetOutcome(ctx, "h1")
	if got.Outcome != hoststore.OutcomeTrashedAtSource {
		t.Fatalf("trash must win: %+v", got)
	}
}

func TestServeKeepDeleteClearsKept(t *testing.T) {
	t.Parallel()
	st, ctx := openKeepTestStore(t)
	if err := st.UpsertOutcome(ctx, hoststore.MessageOutcome{
		ObjectHash: "h1", Outcome: hoststore.OutcomeKept, Source: hoststore.OutcomeSourceOverviewKeep,
	}); err != nil {
		t.Fatal(err)
	}
	srv := (&IntentPageServer{HTML: []byte("h"), Outcomes: stubOutcomes{st: st}}).Handler()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/keep?hash=h1", nil)
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
	_, ok, err := st.GetOutcome(ctx, "h1")
	if err != nil || ok {
		t.Fatalf("kept still present ok=%v err=%v", ok, err)
	}
}

func TestServeKeepUnavailableWithoutStore(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h")}).Handler())
	t.Cleanup(ts.Close)
	res, err := http.Post(ts.URL+"/keep", "application/json", bytes.NewReader([]byte(`{"items":[]}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
}
