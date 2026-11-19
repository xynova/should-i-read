package mailsync

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xynova/should-i-read/internal/hostevent"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const absencesRunTimeout = 2 * time.Hour

// Engine is Neverest sync with an optional --accept-absences retry.
type Engine interface {
	Sync(ctx context.Context, account string, excludeCollections []string) (*pimalaya.Result, error)
	SyncAcceptingAbsences(ctx context.Context, account string, excludeCollections []string) (*pimalaya.Result, error)
}

// OutcomeLookup reads message_outcomes for leftover replica hashes.
type OutcomeLookup interface {
	GetOutcomes(ctx context.Context, hashes []string) (map[string]hoststore.MessageOutcome, error)
}

// FolderHashes lists live pimdir object hashes for a Neverest folder name.
type FolderHashes interface {
	ListLiveObjectHashesForFolder(ctx context.Context, folder string) ([]string, error)
}

// AbsenceRunner implements overview MailSyncer: first Sync, then --accept-absences
// when leftover INBOX (or other folder) hashes are already trashed_at_source.
type AbsenceRunner struct {
	Engine Engine
	Store  OutcomeLookup
	Mail   FolderHashes
	Events *hostevent.Bus
}

type streamHooker interface {
	SetStreamLineHook(func([]byte))
}

// Sync runs Neverest, then retries with --accept-absences when this tool already
// recorded the missing hashes as trashed_at_source.
func (r *AbsenceRunner) Sync(ctx context.Context, account string, excludeCollections []string) (res *pimalaya.Result, err error) {
	const op = "mailsync.AbsenceRunner.Sync"
	if r == nil || r.Engine == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil sync engine")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, absencesRunTimeout)
		defer cancel()
	}

	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailsync")
	ctx, span := tracer.Start(ctx, "mailsync.AbsenceRunner.Sync",
		trace.WithAttributes(
			attribute.String("sync.account", strings.TrimSpace(account)),
			attribute.Bool("sync.accept_absences", false),
		),
	)
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	detach := r.bindStream(ctx)
	defer detach()

	res, err = r.Engine.Sync(ctx, account, excludeCollections)
	if !needsAbsenceRetry(err) {
		return res, err
	}
	known, folders, kerr := r.knownTrashedByFolder(ctx, res)
	if kerr != nil {
		span.RecordError(kerr)
		span.SetAttributes(attribute.String("sync.absences_lookup", "failed"))
		return res, err
	}
	rep := pimalaya.SummarizeNeverestJSON(rawFrom(res))
	accept := ShouldAcceptAbsences(rep.Absences, known)
	span.SetAttributes(
		attribute.Bool("sync.accept_absences_retry", accept),
		attribute.Int("sync.absence_folders", len(rep.Absences)),
		attribute.StringSlice("sync.known_trash_folders", folders),
	)
	if !accept {
		return res, err
	}
	span.SetAttributes(attribute.Bool("sync.accept_absences", true))
	return r.Engine.SyncAcceptingAbsences(ctx, account, excludeCollections)
}

func (r *AbsenceRunner) bindStream(ctx context.Context) func() {
	if r == nil || r.Events == nil {
		return func() {}
	}
	setter, ok := r.Engine.(streamHooker)
	if !ok {
		return func() {}
	}
	setter.SetStreamLineHook(func(line []byte) {
		ev, ok := pimalaya.ParseStreamEvent(line)
		if !ok {
			return
		}
		text := ev.ProgressLine()
		if text == "" {
			return
		}
		r.Events.Publish(ctx, hostevent.Event{
			Source: hostevent.SourceSync,
			Kind:   ev.Name,
			Line:   text,
			Done:   ev.Done,
			Total:  ev.Total,
		})
	})
	return func() { setter.SetStreamLineHook(nil) }
}

func needsAbsenceRetry(err error) bool {
	if err == nil {
		return false
	}
	code, ok := sirerr.AsCode(err)
	return ok && code == sirerr.CodeNeedsReview
}

func rawFrom(res *pimalaya.Result) []byte {
	if res == nil {
		return nil
	}
	if len(res.Raw) > 0 {
		return res.Raw
	}
	return []byte(res.Stdout)
}

func (r *AbsenceRunner) knownTrashedByFolder(ctx context.Context, res *pimalaya.Result) (map[string]int, []string, error) {
	if r.Store == nil || r.Mail == nil {
		return map[string]int{}, nil, nil
	}
	rep := pimalaya.SummarizeNeverestJSON(rawFrom(res))
	known := make(map[string]int, len(rep.Absences))
	var folders []string
	for _, a := range rep.Absences {
		folder := strings.TrimSpace(a.Collection)
		if folder == "" || a.Absent <= 0 {
			continue
		}
		hashes, err := r.Mail.ListLiveObjectHashesForFolder(ctx, folder)
		if err != nil {
			return nil, nil, err
		}
		outcomes, err := r.Store.GetOutcomes(ctx, hashes)
		if err != nil {
			return nil, nil, err
		}
		n := 0
		for _, o := range outcomes {
			if strings.TrimSpace(o.Outcome) == hoststore.OutcomeTrashedAtSource {
				n++
			}
		}
		known[folder] = n
		folders = append(folders, folder)
	}
	return known, folders, nil
}

// ShouldAcceptAbsences is true when every Neverest absence is covered by at least
// as many leftover replica hashes already labeled trashed_at_source.
func ShouldAcceptAbsences(absences []pimalaya.Absence, knownByFolder map[string]int) bool {
	if len(absences) == 0 {
		return false
	}
	explained := 0
	for _, a := range absences {
		if a.Absent <= 0 {
			continue
		}
		folder := strings.TrimSpace(a.Collection)
		if folder == "" {
			return false
		}
		if knownByFolder[folder] < a.Absent {
			return false
		}
		explained++
	}
	return explained > 0
}

// Compile-time check that host Store and pimdir Reader match the runner ports.
var (
	_ OutcomeLookup = (*hoststore.Store)(nil)
	_ FolderHashes  = (*pimdir.Reader)(nil)
	_ Engine        = (*pimalaya.Neverest)(nil)
)
