package cli

import (
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/mailsync"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func newMailCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mail",
		Short: "Mailbox setup, sync, and export",
	}
	cmd.AddCommand(newMailSetupCmd(opts, repoRoot))
	cmd.AddCommand(newMailStatusCmd(opts, repoRoot))
	cmd.AddCommand(newMailReadinessCmd(opts, repoRoot))
	cmd.AddCommand(newMailSyncCmd(opts, repoRoot))
	cmd.AddCommand(newMailExportCmd(opts, repoRoot))
	cmd.AddCommand(newMailReportCmd(opts, repoRoot))
	cmd.AddCommand(newMailShowCmd(opts, repoRoot))
	return cmd
}

func newMailSetupCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		provider  string
		account   string
		email     string
		storeRoot string
		force     bool
		skipLogin bool
		wizard    bool
	)
	cmd := &cobra.Command{
		Use:        "setup",
		Hidden:     true,
		Short:      "Onboard a mailbox (deprecated: use configure)",
		Deprecated: "use should-i-read configure",
		Long:       `Deprecated. Use should-i-read configure instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := writeCLIString(cmd.ErrOrStderr(), deprecationConfigure); err != nil {
				return sirerr.Wrap(err, sirerr.CodeFailed, "cli.mail.setup", "write deprecation")
			}
			return runMailSetup(cmd, repoRoot, mailSetupFlags{
				provider: provider, account: account, email: email,
				storeRoot: storeRoot, force: force, skipLogin: skipLogin,
			}, wizard)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Mail provider: gmail or outlook")
	cmd.Flags().StringVar(&account, "account", "", "Sync account id (default: provider name)")
	cmd.Flags().StringVar(&email, "email", "", "Mailbox address")
	cmd.Flags().StringVar(&storeRoot, "store-root", "", "Override local mail store directory")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing mail-sync.toml")
	cmd.Flags().BoolVar(&skipLogin, "skip-login", false, "Stop after config if mailbox login is missing")
	cmd.Flags().BoolVar(&wizard, "wizard", false, "Run interactive wizard (TTY); ignores prefilled flags except store-root and force")
	return cmd
}

type mailSetupFlags struct {
	provider, account, email, storeRoot string
	force, skipLogin                    bool
}

func runMailSetup(cmd *cobra.Command, repoRoot string, flags mailSetupFlags, wizardFlag bool) error {
	const op = "cli.mail.setup"
	ctx := cmd.Context()
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}

	tty := isTerminal(os.Stdin)
	useWizard := mailSetupUseWizard(tty, wizardFlag, cmd.Flags().Changed("provider"), cmd.Flags().Changed("email"))
	if useWizard {
		var err error
		flags, err = runMailSetupWizard(ctx, flags)
		if err != nil {
			return err
		}
	}

	providerRaw := strings.TrimSpace(flags.provider)
	emailRaw := strings.TrimSpace(flags.email)
	if providerRaw == "" || emailRaw == "" {
		if !tty {
			return sirerr.New(sirerr.CodeInvalid, op, "provider and email required (or run from a TTY for the wizard)")
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
		return sirerr.Wrap(err, sirerr.CodeInvalid, op, "invalid provider")
	}
	hostBin, _ := os.Executable()
	w, err := mailsync.CreateWorkflow(repoRoot, hostBin, mailsync.WorkflowHooks{})
	if err != nil {
		return err
	}
	res, err := w.Setup(ctx, mailsync.SetupOptions{
		Provider:    prov,
		Account:     strings.TrimSpace(flags.account),
		Email:       emailRaw,
		StoreRoot:   strings.TrimSpace(flags.storeRoot),
		Force:       flags.force,
		SkipLogin:   flags.skipLogin,
		HostBin:     hostBin,
		Interactive: tty,
	})
	if err != nil {
		if len(res.Steps) > 0 {
			if wantJSON(cmd) {
				_ = printJSON(cmd.OutOrStdout(), res)
			} else {
				_, _ = io.WriteString(cmd.OutOrStdout(), formatMailSetupResult(res)+"\n")
			}
		}
		return err
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), res)
	}
	_, err = io.WriteString(cmd.OutOrStdout(), formatMailSetupResult(res)+"\n")
	return err
}

func newMailStatusCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show mailbox readiness checklist",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailStatus(cmd, opts, repoRoot)
		},
	}
	return cmd
}

func runMailStatus(cmd *cobra.Command, opts *rootOptions, repoRoot string) error {
	const op = "cli.mail.status"
	ctx := cmd.Context()
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	st, err := mailsync.CollectStatus(ctx, cfg)
	if err != nil {
		return err
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), st)
	}
	if err := writeCLILine(cmd.OutOrStdout(), formatMailStatus(st)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write mail status")
	}
	return nil
}
