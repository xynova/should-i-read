package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
)

type holdDeleter struct {
	block chan struct{}
}

func (h *holdDeleter) MessageDelete(ctx context.Context, folder, envelopeID string) (*pimalaya.Result, error) {
	select {
	case <-h.block:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &pimalaya.Result{Raw: json.RawMessage(`{"action":"moved-to-trash","count":1}`)}, nil
}

func TestCartJobApplyRequiresConfirm(t *testing.T) {
	t.Parallel()
	st, svc := testCartJobServer(t, nil)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)

	jobID := createTestCartJob(t, ts.URL, st)
	res, err := http.Post(ts.URL+"/cart/jobs/"+jobID+"/apply", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestCartJobApplyGET405(t *testing.T) {
	t.Parallel()
	st, svc := testCartJobServer(t, nil)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)

	jobID := createTestCartJob(t, ts.URL, st)
	res, err := http.Get(ts.URL + "/cart/jobs/" + jobID + "/apply")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestCartJobApplyAccepts202(t *testing.T) {
	t.Parallel()
	hold := &holdDeleter{block: make(chan struct{})}
	st, svc := testCartJobServer(t, hold)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)
	t.Cleanup(func() { close(hold.block) })

	jobID := createTestCartJob(t, ts.URL, st)
	body, _ := json.Marshal(cartJobApplyRequest{Confirm: hoststore.DeleteJobConfirmToken})
	res, err := http.Post(ts.URL+"/cart/jobs/"+jobID+"/apply", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out cartJobResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != hoststore.DeleteJobStatusApplying {
		t.Fatalf("%+v", out)
	}
}

func TestCartJobApplyMissingHimalaya503(t *testing.T) {
	t.Parallel()
	st, svc := testCartJobServer(t, nil)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)

	jobID := createTestCartJob(t, ts.URL, st)
	body, _ := json.Marshal(cartJobApplyRequest{Confirm: hoststore.DeleteJobConfirmToken})
	res, err := http.Post(ts.URL+"/cart/jobs/"+jobID+"/apply", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out cartJobResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Error, "Himalaya") {
		t.Fatalf("%+v", out)
	}
	if out.Job == nil || out.Job.Status != hoststore.DeleteJobStatusPlanned {
		t.Fatalf("job %+v", out.Job)
	}
}

func TestCartJobGetRecoversStuckApplying(t *testing.T) {
	t.Parallel()
	st, svc := testCartJobServer(t, nil)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)

	jobID := createTestCartJob(t, ts.URL, st)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	now := time.Now().UTC()
	if err := st.SetDeleteJobStatus(ctx, jobID, hoststore.DeleteJobStatusApplying, "", &now); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(ts.URL + "/cart/jobs/" + jobID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out cartJobResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Job == nil || out.Job.Status != hoststore.DeleteJobStatusPlanned {
		t.Fatalf("%+v", out)
	}
	if !strings.Contains(out.Job.Error, "Himalaya") {
		t.Fatalf("%+v", out)
	}
}

type mapPim struct {
	m map[string]pimdir.EmailSummary
}

func (f *mapPim) SummariesByObjectHash(ctx context.Context, hashes []string) (map[string]pimdir.EmailSummary, error) {
	out := map[string]pimdir.EmailSummary{}
	for _, h := range hashes {
		if s, ok := f.m[h]; ok {
			out[h] = s
		}
	}
	return out, nil
}

func TestCartCheckoutRequiresConfirm(t *testing.T) {
	t.Parallel()
	st, svc := testCartJobServer(t, nil)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/cart/checkout", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestCartCheckoutGET405(t *testing.T) {
	t.Parallel()
	st, svc := testCartJobServer(t, nil)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/cart/checkout")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestCartCheckoutClearsCart(t *testing.T) {
	t.Parallel()
	hold := &holdDeleter{block: make(chan struct{})}
	pim := &mapPim{m: map[string]pimdir.EmailSummary{
		"abc1234efgh5678": {Collection: "imap/INBOX", Seq: 1},
	}}
	st, svc := testCartJobServerPim(t, hold, pim)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)
	t.Cleanup(func() { close(hold.block) })

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := st.AddDeleteCart(ctx, []hoststore.DeleteCartItem{{ObjectHash: "abc1234efgh5678"}}); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(cartJobApplyRequest{Confirm: hoststore.DeleteJobConfirmToken})
	res, err := http.Post(ts.URL+"/cart/checkout", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out cartJobResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Job == nil || out.Job.Status != hoststore.DeleteJobStatusApplying {
		t.Fatalf("%+v", out)
	}
	cart, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart) != 0 {
		t.Fatalf("cart after checkout %+v", cart)
	}
}

func testCartJobServer(t *testing.T, deleter maildelete.TrashDeleter) (*hoststore.Store, *maildelete.Service) {
	t.Helper()
	return testCartJobServerPim(t, deleter, &pimdir.Reader{})
}

func testCartJobServerPim(t *testing.T, deleter maildelete.TrashDeleter, pim maildelete.PimdirSummaries) (*hoststore.Store, *maildelete.Service) {
	t.Helper()
	if pim == nil {
		pim = &pimdir.Reader{}
	}
	path := t.TempDir() + "/ledger.db"
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc, err := maildelete.CreateDeleteJobService(maildelete.DeleteJobConfig{
		Store: st, Pimdir: pim, Deleter: deleter, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return st, svc
}

func createTestCartJob(t *testing.T, _ string, st *hoststore.Store) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_ = st.AddDeleteCart(ctx, []hoststore.DeleteCartItem{{ObjectHash: "abc1234efgh5678"}})
	jobID, err := st.CreateDeleteJobFromCart(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BindDeleteJobItems(ctx, jobID, []hoststore.DeleteJobBinding{
		{ObjectHash: "abc1234efgh5678", Folder: "INBOX", EnvelopeID: "1", Status: hoststore.DeleteJobItemPlanned},
	}); err != nil {
		t.Fatal(err)
	}
	return jobID
}
