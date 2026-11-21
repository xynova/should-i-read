package mailoverview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xynova/should-i-read/internal/hostevent"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailack"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	SyncStatusIdle    = "idle"
	SyncStatusRunning = "running"
	SyncStatusDone    = "done"
	SyncStatusFailed  = "failed"
	syncRunTimeout    = 2 * time.Hour
	syncIDBytes       = 8
)

var (
	errSyncBusy    = errors.New("mail sync is already running")
	errSyncOverlap = errors.New("wait for Trash or Acked apply to finish")
)

// MailSyncer runs Neverest sync against the local replica.
type MailSyncer interface {
	Sync(ctx context.Context, account string, excludeCollections []string) (*pimalaya.Result, error)
}

// SyncConfig wires the overview sync controller.
type SyncConfig struct {
	Runner       MailSyncer
	Account      string
	Excludes     []string
	ApplyBaseCtx context.Context
	Now          func() time.Time
	Jobs         *maildelete.Service
	AckJobs      *mailack.Service
	ApplyingFn   func(context.Context) (bool, error)
	Events       *hostevent.Bus
	AfterSync    func(ctx context.Context, res *pimalaya.Result) (string, error)
}

// SyncSnapshot is the in-memory sync state for HTTP and UI.
type SyncSnapshot struct {
	Status  string `json:"status"`
	RunID   string `json:"run_id,omitempty"`
	Account string `json:"account,omitempty"`
	Error   string `json:"error,omitempty"`
	Fetch   int    `json:"fetch,omitempty"`
	Delete  int    `json:"delete,omitempty"`
	Line    string `json:"line,omitempty"`
}

// SyncController runs one Neverest sync at a time in the background.
type SyncController struct {
	runner       MailSyncer
	account      string
	excludes     []string
	applyBaseCtx context.Context
	now          func() time.Time
	jobs         *maildelete.Service
	ackJobs      *mailack.Service
	applyingFn   func(context.Context) (bool, error)
	events       *hostevent.Bus
	afterSync    func(ctx context.Context, res *pimalaya.Result) (string, error)
	mu           sync.Mutex
	snap         SyncSnapshot
}

// CreateSyncController builds a controller from config.
func CreateSyncController(cfg SyncConfig) (*SyncController, error) {
	const op = "mailoverview.CreateSyncController"
	if cfg.Now == nil || cfg.ApplyBaseCtx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing dependencies")
	}
	c := &SyncController{
		runner:       cfg.Runner,
		account:      strings.TrimSpace(cfg.Account),
		excludes:     append([]string(nil), cfg.Excludes...),
		applyBaseCtx: cfg.ApplyBaseCtx,
		now:          cfg.Now,
		jobs:         cfg.Jobs,
		ackJobs:      cfg.AckJobs,
		events:       cfg.Events,
		afterSync:    cfg.AfterSync,
		snap:         SyncSnapshot{Status: SyncStatusIdle},
	}
	if cfg.ApplyingFn != nil {
		c.applyingFn = cfg.ApplyingFn
	} else {
		c.applyingFn = func(ctx context.Context) (bool, error) {
			del, err := deleteJobApplying(ctx, c.jobs)
			if err != nil {
				return false, err
			}
			if del {
				return true, nil
			}
			return ackJobApplying(ctx, c.ackJobs)
		}
	}
	return c, nil
}

// Snapshot returns a copy of the current sync state.
func (c *SyncController) Snapshot() SyncSnapshot {
	if c == nil {
		return SyncSnapshot{Status: SyncStatusIdle}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snap
}

// Running reports whether a sync is in progress.
func (c *SyncController) Running() bool {
	return c.Snapshot().Status == SyncStatusRunning
}

// Start begins a background Neverest sync.
func (c *SyncController) Start(ctx context.Context) (snap SyncSnapshot, err error) {
	const op = "mailoverview.SyncController.Start"
	if c == nil {
		return SyncSnapshot{}, sirerr.New(sirerr.CodeInvalid, op, "nil controller")
	}
	if c.runner == nil {
		return SyncSnapshot{}, sirerr.New(sirerr.CodeUnavailable, op, "mail sync needs neverest")
	}
	applying, aerr := c.applyingFn(ctx)
	if aerr != nil {
		return SyncSnapshot{}, aerr
	}
	if applying {
		return SyncSnapshot{}, sirerr.Wrap(fmt.Errorf("%w", errSyncOverlap), sirerr.CodeInvalid, op, "cart or ack apply in progress")
	}
	c.mu.Lock()
	if c.snap.Status == SyncStatusRunning {
		c.mu.Unlock()
		return SyncSnapshot{}, sirerr.Wrap(fmt.Errorf("%w", errSyncBusy), sirerr.CodeInvalid, op, "busy")
	}
	runID, ridErr := newSyncRunID()
	if ridErr != nil {
		c.mu.Unlock()
		return SyncSnapshot{}, sirerr.Wrap(ridErr, sirerr.CodeFailed, op, "run id")
	}
	c.snap = SyncSnapshot{
		Status:  SyncStatusRunning,
		RunID:   runID,
		Account: c.account,
		Line:    "Syncing…",
	}
	snap = c.snap
	c.mu.Unlock()
	if c.events != nil {
		c.events.Publish(c.applyBaseCtx, hostevent.Event{
			Source: hostevent.SourceSync,
			Kind:   "started",
			Line:   "Syncing…",
		})
	}
	go c.run(runID)
	return snap, nil
}

func (c *SyncController) run(runID string) {
	var runErr error
	var fetch, del int
	var line string
	finalStatus := SyncStatusDone

	base := c.applyBaseCtx
	ctx, cancel := context.WithTimeout(base, syncRunTimeout)
	defer cancel()
	ctx, span := startSyncSpan(ctx, runID, c.account, c.excludes)
	defer func() {
		setSyncSpanResult(span, finalStatus, fetch, del, line)
		endOverviewSpan(span, runErr)
	}()

	stopSub := c.startSyncEventSubscriber(ctx, runID)
	defer stopSub()

	res, runErr := c.runner.Sync(ctx, c.account, c.excludes)
	fetch, del = syncResultCounts(res)
	line = syncStatusLine(res, runErr)
	if runErr != nil {
		finalStatus = SyncStatusFailed
		c.storeSyncSnapshot(runID, SyncStatusFailed, line, runErr, fetch, del)
		return
	}
	c.setRunningLine(runID, line)
	line, afterErr := c.runAfterSyncHook(ctx, runID, res, line)
	if afterErr != nil {
		runErr = afterErr
		finalStatus = SyncStatusFailed
		c.storeSyncSnapshot(runID, SyncStatusFailed, line, afterErr, fetch, del)
		return
	}
	c.storeSyncSnapshot(runID, SyncStatusDone, line, nil, fetch, del)
}

func (c *SyncController) startSyncEventSubscriber(ctx context.Context, runID string) func() {
	if c == nil || c.events == nil {
		return func() {}
	}
	subCtx, stopSub := context.WithCancel(ctx)
	ch := c.events.Subscribe(subCtx)
	go func() {
		for ev := range ch {
			text := strings.TrimSpace(ev.Line)
			if text == "" {
				continue
			}
			c.mu.Lock()
			if c.snap.RunID == runID && c.snap.Status == SyncStatusRunning {
				c.snap.Line = capSyncLine(text)
			}
			c.mu.Unlock()
		}
	}()
	return stopSub
}

func syncResultCounts(res *pimalaya.Result) (fetch, del int) {
	if res == nil || len(res.Raw) == 0 {
		return 0, 0
	}
	rep := pimalaya.SummarizeNeverestJSON(res.Raw)
	if !rep.Recognized {
		return 0, 0
	}
	return rep.Fetch, rep.Delete
}

func (c *SyncController) storeSyncSnapshot(runID, status, line string, err error, fetch, del int) {
	if c == nil {
		return
	}
	snap := SyncSnapshot{
		Status:  status,
		RunID:   runID,
		Account: c.account,
		Fetch:   fetch,
		Delete:  del,
		Line:    line,
	}
	if err != nil {
		snap.Error = syncSnapshotError(line, err)
	}
	c.mu.Lock()
	c.snap = snap
	c.mu.Unlock()
}

func (c *SyncController) runAfterSyncHook(ctx context.Context, runID string, res *pimalaya.Result, line string) (string, error) {
	if c == nil || c.afterSync == nil {
		return line, nil
	}
	if c.events != nil {
		c.events.Publish(ctx, hostevent.Event{
			Source: hostevent.SourceSync,
			Kind:   "reporting",
			Line:   "Reporting…",
		})
	}
	afterCtx, afterSpan := startAfterSyncSpan(ctx, runID)
	afterLine, afterErr := c.afterSync(afterCtx, res)
	endOverviewSpan(afterSpan, afterErr)
	if strings.TrimSpace(afterLine) != "" {
		line = capSyncLine(afterLine)
	}
	return line, afterErr
}

func (c *SyncController) setRunningLine(runID, line string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snap.RunID == runID && c.snap.Status == SyncStatusRunning {
		c.snap.Line = capSyncLine(line)
	}
}

var (
	syncANSIcsi = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	syncANSIosc = regexp.MustCompile(`\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)
)

const syncLineMaxRunes = 96

func syncStripANSI(s string) string {
	s = syncANSIcsi.ReplaceAllString(s, "")
	s = syncANSIosc.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// FormatSyncLine is the replica closer: nothing new, or fetch and delete counts.
func FormatSyncLine(fetch, del int) string {
	if fetch == 0 && del == 0 {
		return capSyncLine("Already synced.")
	}
	return capSyncLine(fmt.Sprintf("Synced. Fetch %d · delete %d", fetch, del))
}

// FormatSyncReportLine is the closer after replica sync plus report-only classify.
func FormatSyncReportLine(fetch, del, newly int, polypusDown bool) string {
	base := FormatSyncLine(fetch, del)
	if polypusDown {
		return capSyncLine(strings.TrimSuffix(base, ".") + ". Report skipped (Polypus down).")
	}
	if newly <= 0 {
		if fetch == 0 && del == 0 {
			return capSyncLine("Already synced. Report up to date.")
		}
		return capSyncLine(base + ". Report up to date.")
	}
	return capSyncLine(fmt.Sprintf("%s. Classified %d.", strings.TrimSuffix(base, "."), newly))
}

func capSyncLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= syncLineMaxRunes {
		return s
	}
	return string(runes[:syncLineMaxRunes]) + "…"
}

func syncStatusLine(res *pimalaya.Result, err error) string {
	rep := syncEngineReport(res)
	if err == nil {
		return syncStatusLineSuccess(rep)
	}
	if line, ok := syncStatusLineNeedsReview(err, rep); ok {
		return line
	}
	if rep.Recognized {
		return syncStatusLineFromReport(rep)
	}
	if line, ok := syncStatusLineFromJSON(res); ok {
		return line
	}
	return syncStatusLineFallback(err)
}

func syncEngineReport(res *pimalaya.Result) pimalaya.EngineReport {
	if res != nil && len(res.Raw) > 0 {
		return pimalaya.SummarizeNeverestJSON(res.Raw)
	}
	return pimalaya.EngineReport{}
}

func syncStatusLineSuccess(rep pimalaya.EngineReport) string {
	if rep.Recognized {
		return FormatSyncLine(rep.Fetch, rep.Delete)
	}
	return capSyncLine("Synced.")
}

func syncStatusLineNeedsReview(err error, rep pimalaya.EngineReport) (string, bool) {
	code, hasCode := sirerr.AsCode(err)
	if !hasCode || code != sirerr.CodeNeedsReview {
		return "", false
	}
	if rep.Recognized && rep.Conflicts > 0 {
		return capSyncLine(fmt.Sprintf("Sync needs review: %d conflict(s)", rep.Conflicts)), true
	}
	return capSyncLine("Sync needs review"), true
}

func syncStatusLineFromReport(rep pimalaya.EngineReport) string {
	if rep.HunkErrors > 0 {
		return capSyncLine(fmt.Sprintf("Sync failed: %d folder error(s)", rep.HunkErrors))
	}
	line := "Sync failed"
	if rep.Fetch != 0 || rep.Delete != 0 {
		line += fmt.Sprintf(" Fetch %d · delete %d", rep.Fetch, rep.Delete)
	}
	return capSyncLine(line)
}

func syncStatusLineFromJSON(res *pimalaya.Result) (string, bool) {
	if res == nil || len(res.Raw) == 0 {
		return "", false
	}
	var probe struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(res.Raw, &probe) != nil {
		return "", false
	}
	c := strings.TrimSpace(probe.Error.Code)
	if c == "" {
		return "", false
	}
	if c == "conflict" || c == string(sirerr.CodeNeedsReview) {
		return capSyncLine("Sync needs review"), true
	}
	return capSyncLine("Sync failed: " + c), true
}

func syncStatusLineFallback(err error) string {
	fallback := syncCleanRunErr(err)
	if fallback == "" {
		return capSyncLine("Sync failed")
	}
	return capSyncLine("Sync failed: " + fallback)
}

func syncCleanRunErr(err error) string {
	if err == nil {
		return ""
	}
	msg := syncStripANSI(err.Error())
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.TrimSpace(msg)
	msg = strings.TrimPrefix(msg, "pimalaya.RunJSON: ")
	if before, _, ok := strings.Cut(msg, " ["); ok {
		msg = strings.TrimSpace(before)
	}
	if strings.Contains(msg, "Resolved credentials") {
		return ""
	}
	if strings.Contains(msg, "pimalaya CLI needs human review") {
		return ""
	}
	if strings.Contains(msg, `stderr={"event":`) || strings.Contains(msg, "account-started") {
		return ""
	}
	return strings.TrimSpace(msg)
}

func syncSnapshotError(line string, err error) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "Sync needs review") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "Sync needs review"))
		rest = strings.TrimPrefix(rest, ":")
		rest = strings.TrimSpace(rest)
		if rest != "" {
			return capSyncLine(rest)
		}
		return "needs review"
	}
	if strings.HasPrefix(line, "Sync failed") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "Sync failed"))
		rest = strings.TrimPrefix(rest, ":")
		rest = strings.TrimSpace(rest)
		if rest != "" {
			return capSyncLine(rest)
		}
		return "failed"
	}
	out := syncCleanRunErr(err)
	if out == "" {
		return "failed"
	}
	return capSyncLine(out)
}

func deleteJobApplying(ctx context.Context, svc *maildelete.Service) (bool, error) {
	if svc == nil {
		return false, nil
	}
	art, err := svc.LatestJob(ctx)
	if err != nil {
		if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeNotFound {
			return false, nil
		}
		return false, err
	}
	return art.Status == hoststore.DeleteJobStatusApplying, nil
}

func ackJobApplying(ctx context.Context, svc *mailack.Service) (bool, error) {
	if svc == nil {
		return false, nil
	}
	art, err := svc.LatestJob(ctx)
	if err != nil {
		if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeNotFound {
			return false, nil
		}
		return false, err
	}
	return art.Status == hoststore.DeleteJobStatusApplying, nil
}

func newSyncRunID() (string, error) {
	var b [syncIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
