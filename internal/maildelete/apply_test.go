package maildelete

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type fakeDeleter struct {
	script string
}

func (f *fakeDeleter) MessageDelete(ctx context.Context, folder, envelopeID string) (*pimalaya.Result, error) {
	r, err := pimalaya.Create(f.script)
	if err != nil {
		return nil, err
	}
	return r.RunJSON(ctx, "message", "delete", "-m", folder, envelopeID)
}

func TestApplyTrashSuccessRemovesCart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	st, ctx := openTestStore(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "himalaya")
	body := `#!/bin/sh
echo '{"action":"moved-to-trash","count":1}'
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = st.AddDeleteCart(ctx, []hoststore.DeleteCartItem{{ObjectHash: "h1"}})
	jobID, err := st.CreateDeleteJobFromCart(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_ = st.BindDeleteJobItems(ctx, jobID, []hoststore.DeleteJobBinding{
		{ObjectHash: "h1", Folder: "INBOX", EnvelopeID: "1", Status: hoststore.DeleteJobItemPlanned},
	})
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Deleter: &fakeDeleter{script: script},
		Now: time.Now, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(ctx, jobID, hoststore.DeleteJobConfirmToken); err != nil {
		t.Fatal(err)
	}
	cart, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart) != 0 {
		t.Fatalf("cart: %+v", cart)
	}
	job, err := st.GetDeleteJob(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Items[0].Status != hoststore.DeleteJobItemTrashed {
		t.Fatalf("%+v", job.Items[0])
	}
}

func TestParseDeleteReportWrapped(t *testing.T) {
	raw := json.RawMessage(`{"data":{"action":"moved-to-trash","count":1}}`)
	action, n, err := ParseDeleteReport(raw)
	if err != nil || action != "moved-to-trash" || n != 1 {
		t.Fatalf("%q %d %v", action, n, err)
	}
}

func TestApplyDeletedActionKeepsCart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	st, ctx := openTestStore(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "himalaya")
	body := `#!/bin/sh
echo '{"action":"deleted","count":1}'
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = st.AddDeleteCart(ctx, []hoststore.DeleteCartItem{{ObjectHash: "h1"}})
	jobID, err := st.CreateDeleteJobFromCart(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_ = st.BindDeleteJobItems(ctx, jobID, []hoststore.DeleteJobBinding{
		{ObjectHash: "h1", Folder: "INBOX", EnvelopeID: "1", Status: hoststore.DeleteJobItemPlanned},
	})
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Deleter: &fakeDeleter{script: script},
		Now: time.Now, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(ctx, jobID, hoststore.DeleteJobConfirmToken); err != nil {
		t.Fatal(err)
	}
	cart, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart) != 1 {
		t.Fatalf("cart len %d", len(cart))
	}
	job, err := st.GetDeleteJob(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Items[0].Status != hoststore.DeleteJobItemFailed {
		t.Fatalf("%+v", job.Items[0])
	}
}

func TestApplySecondPassSkipsTrashed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	st, ctx := openTestStore(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "himalaya")
	body := `#!/bin/sh
echo '{"action":"moved-to-trash","count":1}'
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = st.AddDeleteCart(ctx, []hoststore.DeleteCartItem{{ObjectHash: "h1"}})
	jobID, err := st.CreateDeleteJobFromCart(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_ = st.BindDeleteJobItems(ctx, jobID, []hoststore.DeleteJobBinding{
		{ObjectHash: "h1", Folder: "INBOX", EnvelopeID: "1", Status: hoststore.DeleteJobItemPlanned},
	})
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Deleter: &fakeDeleter{script: script},
		Now: time.Now, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(ctx, jobID, hoststore.DeleteJobConfirmToken); err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(ctx, jobID, hoststore.DeleteJobConfirmToken); err != nil {
		t.Fatal(err)
	}
}

func TestApplyConfirmMismatch(t *testing.T) {
	st, ctx := openTestStore(t)
	svc, err := CreateDeleteJobService(DeleteJobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Deleter: &fakeDeleter{script: "/bin/false"},
		Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.Apply(ctx, "x", "wrong")
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatal(err)
	}
}

func openTestStore(t *testing.T) (*hoststore.Store, context.Context) {
	t.Helper()
	path := t.TempDir() + "/ledger.db"
	now := time.Now().UTC()
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = st.Close() })
	return st, ctx
}
