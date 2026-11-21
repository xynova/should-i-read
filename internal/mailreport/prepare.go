package mailreport

import (
	"context"
	"path/filepath"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hostevent"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	progressFileName    = "mail-classify-progress.json"
	modelResolveTimeout = 45 * time.Second
	prepareDefaultNowOp = "mailreport.PrepareRunner"
)

// PrepareOpts is host wiring for a classify runner (catalog, probes, store).
type PrepareOpts struct {
	RepoRoot        string
	CatalogPath     string
	NoApply         bool
	SkipAuthorProbe bool
	SkipEmbedProbe  bool
	Events          *hostevent.Bus
	Now             time.Time
}

// PrepareRunner opens the ledger, resolves Polypus models, and builds a Runner.
func PrepareRunner(ctx context.Context, cfg config.Config, opts PrepareOpts) (runner *Runner, err error) {
	const op = prepareDefaultNowOp
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing deadline")
	}
	now := opts.Now.UTC()
	if now.IsZero() {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "now is required")
	}
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailreport")
	ctx, span := tracer.Start(ctx, "mailreport.PrepareRunner",
		trace.WithAttributes(
			attribute.String("taxonomy.strategy", cfg.Taxonomy.Strategy),
			attribute.String("polypus.base_url", cfg.PolypusBaseURL),
		),
	)
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	p := &prepareState{op: op, now: now, cfg: cfg, opts: opts}
	if err = p.loadCatalog(); err != nil {
		return nil, err
	}
	if err = p.connectPolypus(ctx); err != nil {
		return nil, err
	}
	resolveCtx, cancel := context.WithTimeout(ctx, modelResolveTimeout)
	defer cancel()
	if err = p.resolveModels(resolveCtx); err != nil {
		return nil, err
	}
	if err = p.loadTags(); err != nil {
		return nil, err
	}
	if err = p.openStore(ctx); err != nil {
		return nil, err
	}
	handedOff := false
	defer func() {
		if !handedOff && p.store != nil {
			_ = p.store.Close()
		}
	}()
	if err = p.buildHarness(); err != nil {
		return nil, err
	}
	if err = p.buildPostSeats(); err != nil {
		return nil, err
	}
	runner, err = p.createRunner()
	if err != nil {
		return nil, err
	}
	handedOff = true
	return runner, nil
}

func pickOrResolveAuthor(ctx context.Context, prober ChatProber, catalogIDs []string, authorYAML string, skipProbe bool) (string, error) {
	if skipProbe {
		return PickAuthorModel(catalogIDs, authorYAML)
	}
	model, _, err := ResolveAuthorModel(ctx, prober, catalogIDs, authorYAML)
	return model, err
}

func pickOrResolveEmbed(ctx context.Context, fac StropEmbedFactory, baseURL string, catalogIDs []string, embedYAML string, skipProbe bool) (string, error) {
	if skipProbe {
		return PickEmbedModel(catalogIDs, embedYAML)
	}
	return ResolveEmbedModel(ctx, fac, baseURL, catalogIDs, embedYAML)
}

func newAttachEmbedInner(ctx context.Context, fac StropEmbedFactory, baseURL, embedModel string) (harness.Embedder, error) {
	const op = "mailreport.newAttachEmbedInner"
	stropEmb, err := fac.CreateEmbedder(ctx, baseURL, embedModel)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "create embedder")
	}
	chunked, err := CreateChunkingEmbedder(ChunkingEmbedderConfig{
		Inner: NewStropEmbedSeat(stropEmb, embedModel),
	})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "chunking embedder")
	}
	return CreateRetryingEmbedder(RetryingEmbedderConfig{Inner: chunked})
}

func importClassifyProgressOnce(ctx context.Context, store *hoststore.Store, repoRoot string) error {
	const op = "mailreport.importClassifyProgressOnce"
	progress := filepath.Join(repoRoot, "tmp", progressFileName)
	pf, err := LoadProgress(progress)
	if err != nil {
		return err
	}
	if len(pf.Entries) == 0 {
		return nil
	}
	_, err = store.ImportProgress(ctx, ProgressEntriesForImport(pf.Entries))
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "import progress")
	}
	return nil
}

func classifyEvents(events *hostevent.Bus, now time.Time) *hostevent.Bus {
	if events != nil {
		return events
	}
	b, err := hostevent.CreateBus(hostevent.BusConfig{Now: func() time.Time { return now }})
	if err != nil {
		return events
	}
	return b
}
