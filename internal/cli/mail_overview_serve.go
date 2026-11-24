package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hostevent"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailack"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/mailoverview"
	"github.com/xynova/should-i-read/internal/mailsync"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const overviewServeShutdownTimeout = 10 * time.Second

func newMailOverviewServeCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		f    mailOverviewFlags
		addr string
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve mail overview with TLDR and delete-safety guesses (Polypus)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			return runMailOverviewServe(cmd, repoRoot, cfg, f, addr)
		},
	}
	bindMailOverviewFlags(cmd, &f)
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8765", "Listen address (loopback)")
	return cmd
}

func runMailOverviewServe(cmd *cobra.Command, repoRoot string, cfg config.Config, f mailOverviewFlags, addr string) error {
	const op = "cli.mail.overview.serve"
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "addr is empty")
	}
	page, err := buildOverviewPage(cmd.Context(), repoRoot, cfg, f)
	if err != nil {
		return err
	}
	if err := writeOverviewFiles(repoRoot, f.outPath, &page); err != nil {
		return err
	}
	pageSrv, applyCancel, closer, err := newOverviewPageServer(cmd.Context(), repoRoot, cfg, f, page)
	if err != nil {
		return err
	}
	defer closer()
	defer applyCancel()
	return serveOverviewUntilStop(cmd, addr, page, pageSrv, applyCancel)
}

func newOverviewPageServer(parent context.Context, repoRoot string, cfg config.Config, f mailOverviewFlags, page overviewPage) (*mailoverview.IntentPageServer, context.CancelFunc, func(), error) {
	const op = "cli.mail.overview.serve"
	nop := func() {}
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: overviewLedgerPath(f, cfg), Now: time.Now().UTC()})
	if err != nil {
		return nil, nop, nop, wrapCLIHop(err, op, "open ledger for deletion cart")
	}
	closer := func() { _ = st.Close() }
	guesser, err := newOverviewIntentGuesser(cfg)
	if err != nil {
		closer()
		return nil, nop, nop, err
	}
	applyCtx, applyCancel := context.WithTimeout(parent, 2*time.Hour)
	jobs, ackJobs, mail, err := openOverviewJobServices(applyCtx, repoRoot, cfg, st)
	if err != nil {
		applyCancel()
		closer()
		return nil, nop, nop, err
	}
	events, err := hostevent.CreateBus(hostevent.BusConfig{Now: time.Now})
	if err != nil {
		applyCancel()
		closer()
		return nil, nop, nop, wrapCLIHop(err, op, "create event bus")
	}
	var preview mailoverview.MailPreviewer
	if mail != nil {
		preview = mailoverview.PimdirPreview{Reader: mail}
	}
	pageSrv := &mailoverview.IntentPageServer{
		HTML:     []byte(page.HTML),
		Guess:    guesser,
		Cart:     st,
		Jobs:     jobs,
		AckCart:  st,
		AckJobs:  ackJobs,
		Actioned: jobs,
		Outcomes: st,
		Preview:  preview,
		Hashes:   page.Hashes,
		Events:   events,
	}
	if err := wireOverviewAfterSync(pageSrv, applyCtx, parent, repoRoot, cfg, f, st, mail, events, jobs, ackJobs); err != nil {
		applyCancel()
		closer()
		return nil, nop, nop, err
	}
	return pageSrv, applyCancel, closer, nil
}

func overviewLedgerPath(f mailOverviewFlags, cfg config.Config) string {
	ledger := strings.TrimSpace(f.ledger)
	if ledger == "" {
		return cfg.StorePath
	}
	return ledger
}

func newOverviewIntentGuesser(cfg config.Config) (*mailoverview.IntentGuesser, error) {
	const op = "cli.mail.overview.serve"
	modelID := strings.TrimSpace(cfg.PolypusEssenceModel)
	if modelID == "" {
		modelID = strings.TrimSpace(cfg.PolypusClassifyModel)
	}
	client, err := polypus.Create(cfg.PolypusBaseURL, nil)
	if err != nil {
		return nil, wrapCLIHop(err, op, "create polypus client")
	}
	guesser, err := mailoverview.CreateIntentGuesser(mailoverview.IntentConfig{Chat: client, Model: modelID})
	if err != nil {
		return nil, wrapCLIHop(err, op, "create intent guesser")
	}
	return guesser, nil
}

func openOverviewJobServices(applyCtx context.Context, repoRoot string, cfg config.Config, st *hoststore.Store) (*maildelete.Service, *mailack.Service, *pimdir.Reader, error) {
	const op = "cli.mail.overview.serve"
	reader, rerr := pimdir.OpenStore(cfg.Pimalaya.PimdirPath)
	if rerr != nil {
		return nil, nil, nil, nil
	}
	var deleter maildelete.TrashDeleter
	var flagger mailack.FlagAdder
	if h, herr := pimalaya.CreateHimalaya(cfg); herr == nil && h != nil {
		deleter = h
		flagger = h
	}
	jobs, err := maildelete.CreateDeleteJobService(maildelete.DeleteJobConfig{
		Store: st, Pimdir: reader, Deleter: deleter, Now: time.Now,
		RepoRoot: repoRoot, ApplyBaseCtx: applyCtx,
	})
	if err != nil {
		return nil, nil, nil, wrapCLIHop(err, op, "create deletion job service")
	}
	ackJobs, err := mailack.CreateAckJobService(mailack.JobConfig{
		Store: st, Pimdir: reader, Flagger: flagger, Now: time.Now,
		RepoRoot: repoRoot, ApplyBaseCtx: applyCtx,
	})
	if err != nil {
		return nil, nil, nil, wrapCLIHop(err, op, "create ack job service")
	}
	return jobs, ackJobs, reader, nil
}

func wireOverviewAfterSync(
	pageSrv *mailoverview.IntentPageServer,
	applyCtx context.Context,
	serveCtx context.Context,
	repoRoot string,
	cfg config.Config,
	f mailOverviewFlags,
	st *hoststore.Store,
	mail *pimdir.Reader,
	events *hostevent.Bus,
	jobs *maildelete.Service,
	ackJobs *mailack.Service,
) error {
	const op = "cli.mail.overview.serve"
	var nev mailoverview.MailSyncer
	if n, nerr := pimalaya.CreateNeverest(cfg); nerr == nil {
		nev = &mailsync.AbsenceRunner{Engine: n, Store: st, Mail: mail, Events: events}
	}
	after, aerr := mailoverview.CreateAfterSync(mailoverview.AfterSyncConfig{
		Classify: func(ctx context.Context, res *pimalaya.Result) (int, bool, error) {
			art, down, err := runPostSyncClassify(ctx, cfg, repoRoot, res, 8192, cfg.Pimalaya.SyncExcludeCollections, events)
			if err != nil {
				return 0, down, err
			}
			return art.NewlyClassified, down, nil
		},
		Rebuild: func(ctx context.Context) ([]byte, []string, error) {
			next, berr := buildOverviewPage(ctx, repoRoot, cfg, f)
			if berr != nil {
				return nil, nil, berr
			}
			if werr := writeOverviewFiles(repoRoot, f.outPath, &next); werr != nil {
				return nil, nil, werr
			}
			return []byte(next.HTML), next.Hashes, nil
		},
		Page: pageSrv,
	})
	if aerr != nil {
		return wrapCLIHop(aerr, op, "create after-sync")
	}
	syncCtl, err := mailoverview.CreateSyncController(mailoverview.SyncConfig{
		Runner:       nev,
		Account:      cfg.Pimalaya.DefaultAccount,
		Excludes:     cfg.Pimalaya.SyncExcludeCollections,
		ApplyBaseCtx: applyCtx,
		RunBaseCtx:   serveCtx,
		Now:          time.Now,
		Jobs:         jobs,
		AckJobs:      ackJobs,
		Events:       events,
		AfterSync:    after.Run,
	})
	if err != nil {
		return wrapCLIHop(err, op, "create mail sync controller")
	}
	pageSrv.Sync = syncCtl
	return nil
}

func serveOverviewUntilStop(cmd *cobra.Command, addr string, page overviewPage, pageSrv *mailoverview.IntentPageServer, applyCancel context.CancelFunc) error {
	const op = "cli.mail.overview.serve"
	serveCtx, serveCancel := context.WithCancel(cmd.Context())
	defer serveCancel()
	srv := newOverviewHTTPServer(addr, pageSrv.Handler(), serveCtx)
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- srv.ListenAndServe()
	}()
	url := "http://" + addr
	box := clui.FormatBox("Mail overview serve", fmt.Sprintf("Open\n%s\nHighlight then mark deletable\nWrote\n%s", url, page.HTMLPath))
	if err := writeCLI(cmd.OutOrStdout(), "%s\n", box); err != nil {
		return err
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	select {
	case err := <-listenErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "listen")
		}
		return nil
	case <-sigCh:
	case <-cmd.Context().Done():
	}
	return shutdownOverviewHTTP(srv, applyCancel, serveCancel, listenErr)
}

func newOverviewHTTPServer(addr string, handler http.Handler, base context.Context) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		BaseContext:       func(net.Listener) context.Context { return base },
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
}

func shutdownOverviewHTTP(srv *http.Server, applyCancel, serveCancel context.CancelFunc, listenErr <-chan error) (err error) {
	const op = "cli.mail.overview.serve"
	if srv == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil http server")
	}
	if applyCancel != nil {
		applyCancel()
	}
	if serveCancel != nil {
		serveCancel()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), overviewServeShutdownTimeout)
	defer cancel()
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/cli")
	ctx, span := tracer.Start(shutdownCtx, "cli.mail.overview.serve.shutdown",
		trace.WithAttributes(attribute.String("http.addr", srv.Addr)),
	)
	defer func() { endCLISpan(span, err) }()
	err = finishOverviewHTTP(ctx, span, srv, listenErr)
	return err
}

func finishOverviewHTTP(ctx context.Context, span trace.Span, srv *http.Server, listenErr <-chan error) error {
	const op = "cli.mail.overview.serve"
	err := srv.Shutdown(ctx)
	if err != nil {
		closeErr := srv.Close()
		if closeErr != nil {
			return sirerr.Wrap(errors.Join(err, closeErr), sirerr.CodeFailed, op, "shutdown")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "shutdown")
		}
		if span != nil {
			span.SetAttributes(attribute.Bool("shutdown.forced_close", true))
		}
	}
	listen := <-listenErr
	if listen != nil && !errors.Is(listen, http.ErrServerClosed) {
		return sirerr.Wrap(listen, sirerr.CodeFailed, op, "listen after shutdown")
	}
	return nil
}

func endCLISpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
