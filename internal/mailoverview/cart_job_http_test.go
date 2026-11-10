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
	"github.com/xynova/should-i-read/internal/pimdir"
)

func TestCartJobApplyRequiresConfirm(t *testing.T) {
	t.Parallel()
	st, svc := testCartJobServer(t)
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
	st, svc := testCartJobServer(t)
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
	st, svc := testCartJobServer(t)
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: st, Jobs: svc}).Handler())
	t.Cleanup(ts.Close)

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

func testCartJobServer(t *testing.T) (*hoststore.Store, *maildelete.Service) {
	t.Helper()
	path := t.TempDir() + "/ledger.db"
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc, err := maildelete.CreateDeleteJobService(maildelete.DeleteJobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Now: time.Now,
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
