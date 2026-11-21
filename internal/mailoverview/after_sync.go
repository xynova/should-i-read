package mailoverview

import (
	"context"

	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// PageReplacer swaps served overview HTML after a post-sync rebuild.
type PageReplacer interface {
	ReplacePage(html []byte, hashes []string)
}

// AfterSyncConfig wires classify, rebuild, and page replace for post-sync work.
type AfterSyncConfig struct {
	Classify func(ctx context.Context, res *pimalaya.Result) (newly int, polypusDown bool, err error)
	Rebuild  func(ctx context.Context) (html []byte, hashes []string, err error)
	Page     PageReplacer
}

// AfterSync classifies, rebuilds the overview page, and replaces served HTML.
type AfterSync struct {
	classify func(ctx context.Context, res *pimalaya.Result) (newly int, polypusDown bool, err error)
	rebuild  func(ctx context.Context) (html []byte, hashes []string, err error)
	page     PageReplacer
}

// CreateAfterSync builds an AfterSync runner.
func CreateAfterSync(cfg AfterSyncConfig) (*AfterSync, error) {
	const op = "mailoverview.CreateAfterSync"
	if cfg.Classify == nil || cfg.Rebuild == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing after-sync hooks")
	}
	return &AfterSync{
		classify: cfg.Classify,
		rebuild:  cfg.Rebuild,
		page:     cfg.Page,
	}, nil
}

// Run classifies new mail when the sync report is recognized, then rebuilds the page.
func (a *AfterSync) Run(ctx context.Context, res *pimalaya.Result) (line string, err error) {
	const op = "mailoverview.AfterSync.Run"
	if a == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil after-sync")
	}
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	ctx, span := startAfterSyncSpan(ctx, "")
	defer func() { endOverviewSpan(span, err) }()

	fetch, del := 0, 0
	var rep pimalaya.EngineReport
	if res != nil && len(res.Raw) > 0 {
		rep = pimalaya.SummarizeNeverestJSON(res.Raw)
		if rep.Recognized {
			fetch, del = rep.Fetch, rep.Delete
		}
	}
	if !rep.Recognized || rep.DryRun {
		return FormatSyncLine(fetch, del), nil
	}
	newly, down, cerr := a.classify(ctx, res)
	if down {
		return FormatSyncReportLine(fetch, del, 0, true), nil
	}
	if cerr != nil {
		return FormatSyncReportLine(fetch, del, 0, false), cerr
	}
	line = FormatSyncReportLine(fetch, del, newly, false)
	html, hashes, berr := a.rebuild(ctx)
	if berr != nil {
		return line, sirerr.Wrap(berr, sirerr.CodeFailed, op, "rebuild overview page")
	}
	if a.page != nil {
		a.page.ReplacePage(html, hashes)
	}
	return line, nil
}
