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
	"github.com/xynova/should-i-read/internal/mailack"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func testSyncServer(t *testing.T, runner MailSyncer) (*httptest.Server, *SyncController) {
	t.Helper()
	ctx := context.Background()
	ctl, err := CreateSyncController(SyncConfig{
		Runner:       runner,
		Account:      "gmail",
		ApplyBaseCtx: ctx,
		Now:          time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Sync: ctl}).Handler())
	t.Cleanup(ts.Close)
	return ts, ctl
}

func decodeSync(t *testing.T, res *http.Response) syncResponse {
	t.Helper()
	var out syncResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSyncHTTPGetIdle(t *testing.T) {
	t.Parallel()
	ts, _ := testSyncServer(t, &holdSyncer{})
	res, err := http.Get(ts.URL + "/sync")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	out := decodeSync(t, res)
	if !out.OK || out.Status != SyncStatusIdle {
		t.Fatalf("%+v", out)
	}
}

func TestSyncHTTPPostAccepted(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	ts, ctl := testSyncServer(t, &holdSyncer{block: block})
	res, err := http.Post(ts.URL+"/sync", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d", res.StatusCode)
	}
	out := decodeSync(t, res)
	if !out.OK || out.Status != SyncStatusRunning {
		t.Fatalf("%+v", out)
	}
	close(block)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ctl.Snapshot().Status == SyncStatusDone {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("snap %+v", ctl.Snapshot())
}

func TestSyncHTTPPostBusy409(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	ts, _ := testSyncServer(t, &holdSyncer{block: block})
	if _, err := http.Post(ts.URL+"/sync", "application/json", bytes.NewReader([]byte(`{}`))); err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/sync", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status %d", res.StatusCode)
	}
	out := decodeSync(t, res)
	if out.Error != errSyncBusy.Error() {
		t.Fatalf("%+v", out)
	}
	close(block)
}

func TestSyncHTTPPostApplyOverlap409(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctl, err := CreateSyncController(SyncConfig{
		Runner:       &holdSyncer{},
		Account:      "gmail",
		ApplyBaseCtx: ctx,
		Now:          time.Now,
		ApplyingFn: func(context.Context) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Sync: ctl}).Handler())
	t.Cleanup(ts.Close)
	res, err := http.Post(ts.URL+"/sync", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status %d", res.StatusCode)
	}
	out := decodeSync(t, res)
	if out.Error != errSyncOverlap.Error() {
		t.Fatalf("%+v", out)
	}
}

func TestSyncHTTPPostNilRunner503(t *testing.T) {
	t.Parallel()
	ts, _ := testSyncServer(t, nil)
	res, err := http.Post(ts.URL+"/sync", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestSyncHTTPMethodNotAllowed(t *testing.T) {
	t.Parallel()
	ts, _ := testSyncServer(t, &holdSyncer{})
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/sync", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestCartCheckoutConflictWhenSyncRunning(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	ctx := context.Background()
	ctl, err := CreateSyncController(SyncConfig{
		Runner:       &holdSyncer{block: block},
		Account:      "gmail",
		ApplyBaseCtx: ctx,
		Now:          time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, svc := testCartJobServer(t, nil)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc, Sync: ctl}).Handler())
	t.Cleanup(ts.Close)
	if _, err := http.Post(ts.URL+"/sync", "application/json", bytes.NewReader([]byte(`{}`))); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(cartJobApplyRequest{Confirm: hoststore.DeleteJobConfirmToken})
	res, err := http.Post(ts.URL+"/cart/checkout", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out cartJobResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "wait for mail sync to finish" {
		t.Fatalf("%+v", out)
	}
	close(block)
}

func TestAckCheckoutConflictWhenSyncRunning(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	ctx := context.Background()
	ctl, err := CreateSyncController(SyncConfig{
		Runner:       &holdSyncer{block: block},
		Account:      "gmail",
		ApplyBaseCtx: ctx,
		Now:          time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, ackSvc := testAckJobServer(t)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), AckCart: st, AckJobs: ackSvc, Sync: ctl}).Handler())
	t.Cleanup(ts.Close)
	if _, err := http.Post(ts.URL+"/sync", "application/json", bytes.NewReader([]byte(`{}`))); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(cartJobApplyRequest{Confirm: hoststore.AckJobConfirmToken})
	res, err := http.Post(ts.URL+"/ack/checkout", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out ackJobResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "wait for mail sync to finish" {
		t.Fatalf("%+v", out)
	}
	close(block)
}

func TestSyncHTTPGetLineAfterNeedsReview(t *testing.T) {
	raw := json.RawMessage(`{"account":"gmail","outstandingConflicts":2,"item":{"patch":[]}}`)
	syncErr := sirerr.New(sirerr.CodeNeedsReview, "pimalaya.RunJSON", "pimalaya CLI needs human review").
		With("stderr", "Resolved credentials")
	ts, _ := testSyncServer(t, &holdSyncer{raw: raw, err: syncErr})
	if _, err := http.Post(ts.URL+"/sync", "application/json", bytes.NewReader([]byte(`{}`))); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(ts.URL + "/sync")
		if err != nil {
			t.Fatal(err)
		}
		out := decodeSync(t, res)
		_ = res.Body.Close()
		if out.Status == SyncStatusFailed {
			if out.Line != "Sync needs review: 2 conflict(s)" {
				t.Fatalf("line %q", out.Line)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for failed sync")
}

func testAckJobServer(t *testing.T) (*hoststore.Store, *mailack.Service) {
	t.Helper()
	path := t.TempDir() + "/ledger.db"
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ackSvc, err := mailack.CreateAckJobService(mailack.JobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return st, ackSvc
}
