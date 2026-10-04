package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/mailsync"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
	"github.com/xynova/should-i-read/internal/triage"
)

func newPimCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "pim",
		Hidden:     true,
		Deprecated: "use should-i-read mail (readiness, sync, export, show)",
		Short:      "Deprecated mail internals; use mail subcommands",
	}
	cmd.AddCommand(newPimEnsureCmd(repoRoot))
	cmd.AddCommand(newPimConfigureCmd(opts, repoRoot))
	cmd.AddCommand(newPimInitCmd(opts, repoRoot))
	cmd.AddCommand(newPimDoctorCmd(opts, repoRoot))
	cmd.AddCommand(newPimSyncCmd(opts, repoRoot))
	cmd.AddCommand(newPimSnapshotCmd(opts, repoRoot))
	cmd.AddCommand(newPimShowCmd(opts, repoRoot))
	cmd.AddCommand(newPimDigestCheckCmd(opts, repoRoot))
	return cmd
}

func newPimEnsureCmd(repoRoot string) *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Install or verify the mail sync dependency (host-managed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPimEnsure(cmd.Context(), repoRoot, checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check-only", false, "Verify only; do not install")
	return cmd
}

func runPimEnsure(ctx context.Context, repoRoot string, checkOnly bool) error {
	const op = "cli.pim.ensure"
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	script := "install-neverest.sh"
	if checkOnly {
		script = "check-neverest.sh"
	}
	path := filepath.Join(repoRoot, "scripts", script)
	c := exec.CommandContext(ctx, path)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, op, "mail sync ensure failed").With("script", script)
	}
	return nil
}

func newPimConfigureCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		provider  string
		account   string
		email     string
		storeRoot string
		force     bool
		skipInit  bool
	)
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Write mail sync config and patch host operator config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPimConfigure(cmd, repoRoot, opts, pimConfigureFlags{
				provider: provider, account: account, email: email,
				storeRoot: storeRoot, force: force, skipInit: skipInit,
			})
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Mail provider: gmail or outlook")
	cmd.Flags().StringVar(&account, "account", "", "Sync account id (default: provider name)")
	cmd.Flags().StringVar(&email, "email", "", "Mailbox address for IMAP XOAUTH2")
	cmd.Flags().StringVar(&storeRoot, "store-root", "", "Override pimdir store directory")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing mail-sync.toml")
	cmd.Flags().BoolVar(&skipInit, "skip-init", false, "Skip pimdir replica init")
	return cmd
}

type pimConfigureFlags struct {
	provider, account, email, storeRoot string
	force, skipInit                      bool
}

func runPimConfigure(cmd *cobra.Command, repoRoot string, opts *rootOptions, flags pimConfigureFlags) error {
	const op = "cli.pim.configure"
	ctx := cmd.Context()
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	providerRaw := strings.TrimSpace(flags.provider)
	emailRaw := strings.TrimSpace(flags.email)
	if providerRaw == "" || emailRaw == "" {
		if !isTerminal(os.Stdin) {
			return sirerr.New(sirerr.CodeInvalid, op, "provider and email required (or run from a TTY for prompts)")
		}
		var err error
		if providerRaw == "" {
			providerRaw, err = promptLine(cmd.ErrOrStderr(), "Mail provider (gmail/outlook): ")
			if err != nil {
				return err
			}
		}
		if emailRaw == "" {
			emailRaw, err = promptLine(cmd.ErrOrStderr(), "Mailbox email: ")
			if err != nil {
				return err
			}
		}
	}
	prov, err := mailsync.ParseProvider(providerRaw)
	if err != nil {
		return err
	}
	hostBin := filepath.Join(repoRoot, "bin", "should-i-read")
	if _, statErr := os.Stat(hostBin); statErr != nil {
		if exe, exeErr := os.Executable(); exeErr == nil {
			hostBin = exe
		}
	}
	svc, err := mailsync.Create(repoRoot, hostBin, nil)
	if err != nil {
		return err
	}
	res, err := svc.Configure(ctx, mailsync.Options{
		Provider:  prov,
		Account:   flags.account,
		Email:     emailRaw,
		StoreRoot: flags.storeRoot,
		Force:     flags.force,
		HostBin:   hostBin,
		SkipInit:  flags.skipInit,
	})
	if err != nil {
		var partial *mailsync.ConfigureError
		if errors.As(err, &partial) && partial != nil {
			if wantJSON(cmd) {
				_ = printJSON(cmd.OutOrStdout(), partial.Result)
			} else {
				_, _ = io.WriteString(cmd.OutOrStdout(), formatMailConfigureResult(partial.Result)+"\n")
			}
		}
		return err
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), res)
	}
	_, err = io.WriteString(cmd.OutOrStdout(), formatMailConfigureResult(res)+"\n")
	return err
}

func newPimInitCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the local pimdir replica for a sync account",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			acct := strings.TrimSpace(account)
			if acct == "" {
				acct = cfg.Pimalaya.DefaultAccount
			}
			if acct == "" {
				return sirerr.New(sirerr.CodeInvalid, "cli.pim.init", "account required (flag or pimalaya.default_account)")
			}
			nev, err := pimalaya.CreateNeverest(cfg)
			if err != nil {
				return err
			}
			res, err := nev.Init(cmd.Context(), acct)
			return finishPimalaya(cmd, opts, res, err, acct)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Sync account id")
	return cmd
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func promptLine(w io.Writer, label string) (string, error) {
	if w != nil {
		fmt.Fprint(w, label)
	}
	var line string
	if _, err := fmt.Fscanln(os.Stdin, &line); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, "cli.prompt", "read line")
	}
	return strings.TrimSpace(line), nil
}

func newPimDigestCheckCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	return &cobra.Command{
		Use:   "digest-check",
		Short: "Polypus health then emit empty digest scaffold (report-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			client, err := polypus.Create(cfg.PolypusBaseURL, nil)
			if err != nil {
				return err
			}
			if _, err := client.Check(cmd.Context()); err != nil {
				return err
			}
			art := triage.DigestArtifact{
				Mode:           "report_only",
				PolypusBaseURL: cfg.PolypusBaseURL,
				GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
				MustRead:       []triage.DigestEntry{},
				FYI:            []triage.DigestEntry{},
				Unwanted:       []triage.DigestEntry{},
			}
			if wantJSON(cmd) {
				return printJSON(cmd.OutOrStdout(), art)
			}
			fmt.Fprintln(cmd.OutOrStdout(), clui.FormatBox("Digest", "Report-only scaffold (empty)\n"+clui.Muted("mode report_only")))
			return nil
		},
	}
}

func newPimDoctorCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	return &cobra.Command{
		Use:        "doctor",
		Hidden:     true,
		Deprecated: "use should-i-read mail readiness",
		Short:      "Check mail sync readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailReadiness(cmd, opts, repoRoot)
		},
	}
}

func newPimSyncCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:        "sync",
		Hidden:     true,
		Deprecated: "use should-i-read mail sync",
		Short:      "Sync mail into the local pimdir store",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailSync(cmd, opts, repoRoot, account)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Mail account id from sync config")
	return cmd
}

func newPimSnapshotCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		limit   int
		outPath string
		store   string
	)
	cmd := &cobra.Command{
		Use:        "snapshot",
		Hidden:     true,
		Deprecated: "use should-i-read mail export",
		Short:      "Export recent mail summaries from pimdir SQLite (report-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			dir := store
			if dir == "" {
				dir = cfg.Pimalaya.PimdirPath
			}
			return runMailExport(cmd, repoRoot, dir, limit, outPath, cfg.PolypusBaseURL)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max messages")
	cmd.Flags().StringVar(&outPath, "out", "", "Output path (default tmp/pim-snapshot-<ts>.json)")
	cmd.Flags().StringVar(&store, "store", "", "Pimdir store directory (overrides config)")
	return cmd
}

func newPimShowCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		store    string
		raw      bool
		maxBody  int
	)
	cmd := &cobra.Command{
		Use:        "show <ref>",
		Hidden:     true,
		Deprecated: "use should-i-read mail show",
		Short:      "Show one synced message body from the local pimdir store",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			dir := store
			if dir == "" {
				dir = cfg.Pimalaya.PimdirPath
			}
			return runMailShow(cmd.Context(), cmd.OutOrStdout(), dir, args[0], raw, maxBody)
		},
	}
	cmd.Flags().StringVar(&store, "store", "", "Pimdir store directory (overrides config)")
	cmd.Flags().BoolVar(&raw, "raw", false, "Write full RFC822 blob bytes to stdout")
	cmd.Flags().IntVar(&maxBody, "max-body", 8192, "Max body bytes in default preview mode")
	return cmd
}

