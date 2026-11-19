package mailsync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/xynova/should-i-read/internal/hostevent"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestShouldAcceptAbsences(t *testing.T) {
	t.Parallel()
	abs := []pimalaya.Absence{{Collection: "INBOX", Absent: 1054, Bound: 2069}}
	if ShouldAcceptAbsences(nil, map[string]int{"INBOX": 1054}) {
		t.Fatal("empty absences")
	}
	if ShouldAcceptAbsences(abs, map[string]int{"INBOX": 10}) {
		t.Fatal("known trash below absent")
	}
	if !ShouldAcceptAbsences(abs, map[string]int{"INBOX": 1054}) {
		t.Fatal("known trash covers INBOX")
	}
	if ShouldAcceptAbsences(abs, map[string]int{"Sent": 1054}) {
		t.Fatal("wrong folder")
	}
}

type fakeEngine struct {
	firstRes  *pimalaya.Result
	firstErr  error
	secondRes *pimalaya.Result
	secondErr error
	syncs     int
	accepts   int
}

func (f *fakeEngine) Sync(ctx context.Context, account string, excludeCollections []string) (*pimalaya.Result, error) {
	f.syncs++
	return f.firstRes, f.firstErr
}

func (f *fakeEngine) SyncAcceptingAbsences(ctx context.Context, account string, excludeCollections []string) (*pimalaya.Result, error) {
	f.accepts++
	return f.secondRes, f.secondErr
}

type fakeOutcomes struct {
	byHash map[string]hoststore.MessageOutcome
	err    error
}

func (f fakeOutcomes) GetOutcomes(ctx context.Context, hashes []string) (map[string]hoststore.MessageOutcome, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]hoststore.MessageOutcome{}
	for _, h := range hashes {
		if o, ok := f.byHash[h]; ok {
			out[h] = o
		}
	}
	return out, nil
}

type fakeFolders struct {
	hashes []string
	err    error
}

func (f fakeFolders) ListLiveObjectHashesForFolder(ctx context.Context, folder string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]string(nil), f.hashes...), nil
}

func inboxAbsenceRaw() []byte {
	return []byte(`{"accounts":[{"report":{"account":"gmail","item":{"patch":[]},"outstandingConflicts":0,"absences":[{"side":"imap","collection":"INBOX","absent":2,"bound":4,"rebuild":false}]}}]}`)
}

func TestAbsenceRunnerRetriesWhenTrashCoversAbsences(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	eng := &fakeEngine{
		firstRes:  &pimalaya.Result{Raw: inboxAbsenceRaw()},
		firstErr:  sirerr.New(sirerr.CodeNeedsReview, "test", "needs review"),
		secondRes: &pimalaya.Result{OK: true, Raw: []byte(`{"account":"gmail","item":{"patch":[]}}`)},
	}
	runner := &AbsenceRunner{
		Engine: eng,
		Store: fakeOutcomes{byHash: map[string]hoststore.MessageOutcome{
			"h1": {ObjectHash: "h1", Outcome: hoststore.OutcomeTrashedAtSource},
			"h2": {ObjectHash: "h2", Outcome: hoststore.OutcomeTrashedAtSource},
		}},
		Mail: fakeFolders{hashes: []string{"h1", "h2"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := runner.Sync(ctx, "gmail", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("res %+v", res)
	}
	if eng.syncs != 1 || eng.accepts != 1 {
		t.Fatalf("syncs=%d accepts=%d", eng.syncs, eng.accepts)
	}
	found := false
	for _, sp := range sr.Ended() {
		if sp.Name() == "mailsync.AbsenceRunner.Sync" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing mailsync.AbsenceRunner.Sync span")
	}
}

func TestAbsenceRunnerSkipsRetryWhenTrashDoesNotCover(t *testing.T) {
	eng := &fakeEngine{
		firstRes: &pimalaya.Result{Raw: inboxAbsenceRaw()},
		firstErr: sirerr.New(sirerr.CodeNeedsReview, "test", "needs review"),
	}
	runner := &AbsenceRunner{
		Engine: eng,
		Store: fakeOutcomes{byHash: map[string]hoststore.MessageOutcome{
			"h1": {ObjectHash: "h1", Outcome: hoststore.OutcomeTrashedAtSource},
		}},
		Mail: fakeFolders{hashes: []string{"h1"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := runner.Sync(ctx, "gmail", nil)
	if err == nil {
		t.Fatal("expected needs review")
	}
	if c, ok := sirerr.AsCode(err); !ok || c != sirerr.CodeNeedsReview {
		t.Fatalf("err %v", err)
	}
	if eng.accepts != 0 {
		t.Fatalf("accepts %d", eng.accepts)
	}
}

type hookEngine struct {
	fakeEngine
	hook func([]byte)
}

func (h *hookEngine) SetStreamLineHook(fn func([]byte)) {
	h.hook = fn
}

func (h *hookEngine) Sync(ctx context.Context, account string, excludeCollections []string) (*pimalaya.Result, error) {
	if h.hook != nil {
		h.hook([]byte(`{"event":"scan-progressed","source":"imap","done":1,"total":9}`))
	}
	return h.fakeEngine.Sync(ctx, account, excludeCollections)
}

func TestAbsenceRunnerPublishesScanProgress(t *testing.T) {
	t.Parallel()
	bus := hostevent.CreateBus()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch := bus.Subscribe(ctx)
	eng := &hookEngine{
		fakeEngine: fakeEngine{
			firstRes: &pimalaya.Result{OK: true, Raw: []byte(`{"account":"gmail","item":{"patch":[]}}`)},
		},
	}
	runner := &AbsenceRunner{Engine: eng, Events: bus}
	if _, err := runner.Sync(ctx, "gmail", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		if ev.Source != hostevent.SourceSync || ev.Line != "Scanning imap 1/9" {
			t.Fatalf("%+v", ev)
		}
	case <-ctx.Done():
		t.Fatal("missing scan event")
	}
}

func TestAbsenceRunnerLookupErrorKeepsFirstReview(t *testing.T) {
	eng := &fakeEngine{
		firstRes: &pimalaya.Result{Raw: inboxAbsenceRaw()},
		firstErr: sirerr.New(sirerr.CodeNeedsReview, "test", "needs review"),
	}
	runner := &AbsenceRunner{
		Engine: eng,
		Store:  fakeOutcomes{err: errors.New("ledger down")},
		Mail:   fakeFolders{hashes: []string{"h1", "h2"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := runner.Sync(ctx, "gmail", nil)
	if err == nil || !strings.Contains(err.Error(), "needs review") {
		t.Fatalf("err %v", err)
	}
	if eng.accepts != 0 {
		t.Fatalf("accepts %d", eng.accepts)
	}
}
