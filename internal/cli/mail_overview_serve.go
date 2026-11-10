package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/mailoverview"
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
	ledger := strings.TrimSpace(f.ledger)
	if ledger == "" {
		ledger = cfg.StorePath
	}
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: ledger, Now: time.Now().UTC()})
	if err != nil {
		return wrapCLIHop(err, op, "open ledger for deletion cart")
	}
	defer func() { _ = st.Close() }()
	modelID := strings.TrimSpace(cfg.PolypusEssenceModel)
	if modelID == "" {
		modelID = strings.TrimSpace(cfg.PolypusClassifyModel)
	}
	client, err := polypus.Create(cfg.PolypusBaseURL, nil)
	if err != nil {
		return wrapCLIHop(err, op, "create polypus client")
	}
	guesser, err := mailoverview.CreateIntentGuesser(mailoverview.IntentConfig{Chat: client, Model: modelID})
	if err != nil {
		return wrapCLIHop(err, op, "create intent guesser")
	}
	applyCtx, applyCancel := context.WithCancel(context.Background())
	defer applyCancel()
	var jobs *maildelete.Service
	if reader, rerr := pimdir.OpenStore(cfg.Pimalaya.PimdirPath); rerr == nil {
		var deleter maildelete.TrashDeleter
		if h, herr := pimalaya.CreateHimalaya(cfg); herr == nil && h != nil {
			deleter = h
		}
		jobs, err = maildelete.CreateDeleteJobService(maildelete.DeleteJobConfig{
			Store: st, Pimdir: reader, Deleter: deleter, Now: time.Now,
			RepoRoot: repoRoot, ApplyBaseCtx: applyCtx,
		})
		if err != nil {
			return wrapCLIHop(err, op, "create deletion job service")
		}
	}
	handler := (&mailoverview.IntentPageServer{
		HTML:  []byte(page.HTML),
		Guess: guesser,
		Cart:  st,
		Jobs:  jobs,
	}).Handler()
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
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

	applyCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), overviewServeShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "shutdown")
	}
	err = <-listenErr
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "listen after shutdown")
	}
	return nil
}
