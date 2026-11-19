package mailjob

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type memStore struct {
	mu         sync.Mutex
	job        Job
	setItemErr error
}

func (m *memStore) SetJobStatus(_ context.Context, jobID, status, errText string, _ *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.job.ID = jobID
	m.job.Status = status
	m.job.Error = errText
	return nil
}

func (m *memStore) ListApplyableItems(_ context.Context, _ string) ([]Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Item, 0, len(m.job.Items))
	for _, it := range m.job.Items {
		if it.Status == hoststore.AckJobItemPlanned || it.Status == hoststore.DeleteJobItemPlanned {
			out = append(out, it)
		}
	}
	return out, nil
}

func (m *memStore) BindItems(_ context.Context, _ string, bindings []Binding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range bindings {
		for i, it := range m.job.Items {
			if it.ObjectHash != b.ObjectHash {
				continue
			}
			m.job.Items[i].Folder = b.Folder
			m.job.Items[i].EnvelopeID = b.EnvelopeID
			m.job.Items[i].Status = b.Status
			m.job.Items[i].Error = b.Error
		}
	}
	return nil
}

func (m *memStore) SetItemStatus(_ context.Context, _, objectHash, status, errText string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setItemErr != nil {
		return m.setItemErr
	}
	for i, it := range m.job.Items {
		if it.ObjectHash != objectHash {
			continue
		}
		m.job.Items[i].Status = status
		m.job.Items[i].Error = errText
	}
	return nil
}

func (m *memStore) GetJob(_ context.Context, jobID string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.job
	out.ID = jobID
	items := append([]Item(nil), m.job.Items...)
	out.Items = items
	return out, nil
}

func (m *memStore) UpsertOutcome(_ context.Context, _ hoststore.MessageOutcome) error {
	return nil
}

func (m *memStore) RemoveCart(_ context.Context, _ string) error {
	return nil
}

type stubResolver struct{}

func (stubResolver) Resolve(_ context.Context, _ []string) ([]Binding, error) {
	return nil, nil
}

type recordingHopper struct {
	folder string
	env    string
	err    error
}

func (h *recordingHopper) Hop(_ context.Context, folder, envelopeID string) error {
	h.folder = folder
	h.env = envelopeID
	return h.err
}

func testApplyCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestApplyPersistsSuccess(t *testing.T) {
	ctx := testApplyCtx(t)
	st := &memStore{job: Job{
		ID:     "j1",
		Status: hoststore.DeleteJobStatusPlanned,
		Items: []Item{{
			ObjectHash: "h1", Sender: "a@x", Folder: "INBOX", EnvelopeID: "1",
			Status: hoststore.AckJobItemPlanned,
		}},
	}}
	hop := &recordingHopper{}
	r, err := CreateRunner(Config{
		Store: st, Resolver: stubResolver{}, Hopper: hop, Spec: SeenSpec(),
		Now: time.Now, ApplyBaseCtx: ctx, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(ctx, "j1", hoststore.AckJobConfirmToken); err != nil {
		t.Fatal(err)
	}
	if hop.folder != "INBOX" || hop.env != "1" {
		t.Fatalf("hop %+v", hop)
	}
	if st.job.Items[0].Status != hoststore.AckJobItemSeen {
		t.Fatalf("item %+v", st.job.Items[0])
	}
	if st.job.Status != hoststore.DeleteJobStatusDone {
		t.Fatalf("job status %q", st.job.Status)
	}
}

func TestApplyReturnsStoreErrorAfterHop(t *testing.T) {
	ctx := testApplyCtx(t)
	st := &memStore{
		setItemErr: errors.New("ledger down"),
		job: Job{
			ID: "j1",
			Items: []Item{{
				ObjectHash: "h1", Folder: "INBOX", EnvelopeID: "1",
				Status: hoststore.AckJobItemPlanned,
			}},
		},
	}
	r, err := CreateRunner(Config{
		Store: st, Resolver: stubResolver{}, Hopper: &recordingHopper{}, Spec: SeenSpec(),
		Now: time.Now, ApplyBaseCtx: ctx, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = r.Apply(ctx, "j1", hoststore.AckJobConfirmToken)
	if err == nil {
		t.Fatal("expected store error")
	}
}

func TestCreateRunnerRequiresDeadline(t *testing.T) {
	t.Parallel()
	_, err := CreateRunner(Config{
		Store: &memStore{}, Resolver: stubResolver{}, Spec: SeenSpec(),
		Now: time.Now, ApplyBaseCtx: context.Background(),
	})
	if err == nil {
		t.Fatal("expected missing deadline")
	}
	if code, ok := sirerr.AsCode(err); !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code %v %v", ok, err)
	}
}

func TestApplyRejectsWrongConfirm(t *testing.T) {
	ctx := testApplyCtx(t)
	r, err := CreateRunner(Config{
		Store: &memStore{}, Resolver: stubResolver{}, Hopper: &recordingHopper{}, Spec: SeenSpec(),
		Now: time.Now, ApplyBaseCtx: ctx, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(ctx, "j1", hoststore.DeleteJobConfirmToken); err == nil {
		t.Fatal("expected confirm mismatch")
	}
}
