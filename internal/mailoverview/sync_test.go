package mailoverview

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/xynova/should-i-read/internal/hostevent"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type holdSyncer struct {
	block chan struct{}
	raw   json.RawMessage
	err   error
}

func (h *holdSyncer) Sync(ctx context.Context, account string, excludeCollections []string) (*pimalaya.Result, error) {
	if h.block != nil {
		select {
		case <-h.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var res *pimalaya.Result
	switch {
	case h.raw != nil:
		res = &pimalaya.Result{Raw: h.raw}
	case h.err == nil:
		res = &pimalaya.Result{Raw: json.RawMessage(`{"account":"gmail","item":{"patch":[]}}`)}
	default:
		res = &pimalaya.Result{}
	}
	if h.err != nil {
		return res, h.err
	}
	res.OK = true
	return res, nil
}

func testSyncController(t *testing.T, runner MailSyncer, applyingFn func(context.Context) (bool, error)) *SyncController {
	t.Helper()
	ctx := context.Background()
	ctl, err := CreateSyncController(SyncConfig{
		Runner:       runner,
		Account:      "gmail",
		Excludes:     []string{"[Gmail]/Spam"},
		ApplyBaseCtx: ctx,
		Now:          time.Now,
		ApplyingFn:   applyingFn,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ctl
}

func TestSyncControllerStartRunsOnce(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	ctl := testSyncController(t, &holdSyncer{block: block}, nil)
	ctx := context.Background()
	snap, err := ctl.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Status != SyncStatusRunning {
		t.Fatalf("snap %+v", snap)
	}
	close(block)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ctl.Snapshot().Status == SyncStatusDone {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("final %+v", ctl.Snapshot())
}

func TestSyncControllerLineFromEvents(t *testing.T) {
	t.Parallel()
	bus, berr := hostevent.CreateBus(hostevent.BusConfig{Now: time.Now})
	if berr != nil {
		t.Fatal(berr)
	}
	block := make(chan struct{})
	ctl, err := CreateSyncController(SyncConfig{
		Runner:       &holdSyncer{block: block},
		Account:      "gmail",
		ApplyBaseCtx: context.Background(),
		Now:          time.Now,
		Events:       bus,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ctl.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ctl.Snapshot().Status == SyncStatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for time.Now().Before(deadline) {
		bus.Publish(context.Background(), hostevent.Event{Line: "Scanning imap 3/9"})
		if ctl.Snapshot().Line == "Scanning imap 3/9" {
			close(block)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(block)
	t.Fatalf("line %q", ctl.Snapshot().Line)
}

func TestFormatSyncLineAlreadySynced(t *testing.T) {
	t.Parallel()
	if got := FormatSyncLine(0, 0); got != "Already synced." {
		t.Fatalf("got %q", got)
	}
	if got := FormatSyncReportLine(0, 0, 0, false); got != "Already synced. Report up to date." {
		t.Fatalf("got %q", got)
	}
	if got := FormatSyncReportLine(0, 0, 3, false); got != "Already synced. Classified 3." {
		t.Fatalf("got %q", got)
	}
}

func TestSyncControllerAfterSyncRuns(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	var called atomic.Bool
	ctl, err := CreateSyncController(SyncConfig{
		Runner:       &holdSyncer{block: block},
		Account:      "gmail",
		ApplyBaseCtx: context.Background(),
		Now:          time.Now,
		AfterSync: func(ctx context.Context, res *pimalaya.Result) (string, error) {
			called.Store(true)
			if res == nil {
				t.Fatal("nil result")
			}
			return "Already synced. Report up to date.", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ctl.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(block)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snap := ctl.Snapshot()
		if snap.Status == SyncStatusDone {
			if !called.Load() {
				t.Fatal("AfterSync not called")
			}
			if snap.Line != "Already synced. Report up to date." {
				t.Fatalf("line %q", snap.Line)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("final %+v called=%v", ctl.Snapshot(), called.Load())
}

func TestSyncControllerAfterSyncSkippedOnError(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	ctl, err := CreateSyncController(SyncConfig{
		Runner:       &holdSyncer{err: sirerr.New(sirerr.CodeNeedsReview, "test", "needs review")},
		Account:      "gmail",
		ApplyBaseCtx: context.Background(),
		Now:          time.Now,
		AfterSync: func(ctx context.Context, res *pimalaya.Result) (string, error) {
			called.Store(true)
			return "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ctl.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ctl.Snapshot().Status == SyncStatusFailed {
			if called.Load() {
				t.Fatal("AfterSync ran after sync failure")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("final %+v", ctl.Snapshot())
}

func TestSyncControllerStartBusy(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	ctl := testSyncController(t, &holdSyncer{block: block}, nil)
	ctx := context.Background()
	if _, err := ctl.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, err := ctl.Start(ctx)
	if err == nil {
		t.Fatal("expected busy")
	}
	if !errors.Is(err, errSyncBusy) {
		t.Fatalf("err %v", err)
	}
	close(block)
}

func TestSyncControllerNilRunner(t *testing.T) {
	t.Parallel()
	ctl := testSyncController(t, nil, nil)
	_, err := ctl.Start(context.Background())
	if err == nil {
		t.Fatal("expected unavailable")
	}
	if c, ok := sirerr.AsCode(err); !ok || c != sirerr.CodeUnavailable {
		t.Fatalf("err %v", err)
	}
}

func TestSyncControllerApplyingOverlap(t *testing.T) {
	t.Parallel()
	ctl := testSyncController(t, &holdSyncer{}, func(context.Context) (bool, error) {
		return true, nil
	})
	_, err := ctl.Start(context.Background())
	if err == nil {
		t.Fatal("expected overlap")
	}
	if !errors.Is(err, errSyncOverlap) {
		t.Fatalf("err %v", err)
	}
}

func TestSyncControllerRunSyncSpanSuccess(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	ctl := testSyncController(t, &holdSyncer{}, nil)
	if _, err := ctl.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ctl.Snapshot().Status == SyncStatusDone {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var runSpan sdktrace.ReadOnlySpan
	for _, sp := range sr.Ended() {
		if sp.Name() == "mailoverview.RunSync" {
			runSpan = sp
			break
		}
	}
	if runSpan == nil {
		t.Fatal("missing mailoverview.RunSync span")
	}
	attrs := attribute.NewSet(runSpan.Attributes()...)
	gotAccount, _ := attrs.Value(attribute.Key("sync.account"))
	if gotAccount.AsString() != "gmail" {
		t.Fatalf("account %q", gotAccount.AsString())
	}
	if runSpan.Status().Code == codes.Error {
		t.Fatalf("status %+v", runSpan.Status())
	}
	if ctl.Snapshot().Line != "Already synced." {
		t.Fatalf("line %q", ctl.Snapshot().Line)
	}
}

func TestSyncControllerRunSyncSpanError(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	ctl := testSyncController(t, &holdSyncer{err: errors.New("neverest boom")}, nil)
	if _, err := ctl.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ctl.Snapshot().Status == SyncStatusFailed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var runSpan sdktrace.ReadOnlySpan
	for _, sp := range sr.Ended() {
		if sp.Name() == "mailoverview.RunSync" {
			runSpan = sp
			break
		}
	}
	if runSpan == nil {
		t.Fatal("missing mailoverview.RunSync span")
	}
	if runSpan.Status().Code != codes.Error {
		t.Fatalf("status %+v", runSpan.Status())
	}
}

func TestSyncControllerConcurrentStart(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	ctl := testSyncController(t, &holdSyncer{block: block}, nil)
	ctx := context.Background()
	var wg sync.WaitGroup
	starts := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(ix int) {
			defer wg.Done()
			_, starts[ix] = ctl.Start(ctx)
		}(i)
	}
	wg.Wait()
	var ok, busy int
	for _, err := range starts {
		if err == nil {
			ok++
			continue
		}
		if errors.Is(err, errSyncBusy) {
			busy++
		}
	}
	if ok != 1 || busy != 1 {
		t.Fatalf("starts %+v", starts)
	}
	close(block)
}

func TestSyncControllerLineNeedsReviewFromJSON(t *testing.T) {
	raw := json.RawMessage(`{"account":"gmail","outstandingConflicts":2,"item":{"patch":[]}}`)
	syncErr := sirerr.New(sirerr.CodeNeedsReview, "pimalaya.RunJSON", "pimalaya CLI needs human review").
		With("stderr", "Resolved credentials")
	ctl := testSyncController(t, &holdSyncer{raw: raw, err: syncErr}, nil)
	if _, err := ctl.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snap := ctl.Snapshot()
		if snap.Status == SyncStatusFailed {
			if snap.Line != "Sync needs review: 2 conflict(s)" {
				t.Fatalf("line %q", snap.Line)
			}
			if strings.Contains(snap.Error, "\x1b") || strings.Contains(snap.Error, "Resolved credentials") {
				t.Fatalf("error %q", snap.Error)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("snap %+v", ctl.Snapshot())
}

func TestSyncControllerLineNeedsReviewEmptyStdout(t *testing.T) {
	syncErr := sirerr.New(sirerr.CodeNeedsReview, "pimalaya.RunJSON", "pimalaya CLI needs human review").
		With("exit_code", "2").
		With("stderr", `{"event":"account-started","account":"gmail"}`)
	ctl := testSyncController(t, &holdSyncer{err: syncErr}, nil)
	if _, err := ctl.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snap := ctl.Snapshot()
		if snap.Status == SyncStatusFailed {
			if snap.Line != "Sync needs review" {
				t.Fatalf("line %q", snap.Line)
			}
			if strings.Contains(snap.Line, "exit_code") || strings.Contains(snap.Line, "account-started") {
				t.Fatalf("line %q", snap.Line)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("snap %+v", ctl.Snapshot())
}

func TestSyncControllerLineNeedsReviewZeroConflicts(t *testing.T) {
	raw := json.RawMessage(`{"account":"gmail","outstandingConflicts":0,"item":{"patch":[]}}`)
	syncErr := sirerr.New(sirerr.CodeNeedsReview, "pimalaya.RunJSON", "pimalaya CLI needs human review")
	ctl := testSyncController(t, &holdSyncer{raw: raw, err: syncErr}, nil)
	if _, err := ctl.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ctl.Snapshot().Status == SyncStatusFailed {
			if ctl.Snapshot().Line != "Sync needs review" {
				t.Fatalf("line %q", ctl.Snapshot().Line)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("snap %+v", ctl.Snapshot())
}
