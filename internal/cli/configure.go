package cli

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/configure"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func newConfigureCmd(rootOpts *rootOptions, repoRoot string) *cobra.Command {
	var (
		apply     bool
		step      string
		provider  string
		account   string
		email     string
		storeRoot string
		force     bool
		skipLogin bool
	)
	cmd := &cobra.Command{
		Use:   VerbConfigure,
		Short: "Operator onboarding hub (config, OAuth apps, mailbox)",
		Long: `Interactive operator configure: host config, OAuth app credentials, mailbox sync,
login, and local store. TTY shows a hub menu to run or fix steps. Use --json, --apply,
or --step for automation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigure(cmd, repoRoot, rootOpts, configureFlags{
				apply: apply, step: step,
				provider: provider, account: account, email: email,
				storeRoot: storeRoot, force: force, skipLogin: skipLogin,
			})
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Run all missing steps")
	cmd.Flags().StringVar(&step, "step", "", "Run one step (see help)")
	cmd.Flags().StringVar(&provider, "provider", "", "Mail provider: gmail or outlook")
	cmd.Flags().StringVar(&account, "account", "", "Sync account id")
	cmd.Flags().StringVar(&email, "email", "", "Mailbox email")
	cmd.Flags().StringVar(&storeRoot, "store-root", "", "Override local mail store directory")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing mail-sync.toml and himalaya.toml")
	cmd.Flags().BoolVar(&skipLogin, "skip-login", false, "Stop before mailbox login on --apply")
	return cmd
}

type configureFlags struct {
	apply, force, skipLogin                   bool
	step, provider, account, email, storeRoot string
}

func runConfigure(cmd *cobra.Command, repoRoot string, rootOpts *rootOptions, flags configureFlags) error {
	const op = "cli.configure"
	ctx := cmd.Context()
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	tty := isTerminal(os.Stdin)
	opts := configure.Options{
		Provider:    strings.TrimSpace(flags.provider),
		Email:       strings.TrimSpace(flags.email),
		Account:     strings.TrimSpace(flags.account),
		StoreRoot:   strings.TrimSpace(flags.storeRoot),
		Force:       flags.force,
		SkipLogin:   flags.skipLogin,
		Interactive: tty,
	}
	hostBin, _ := os.Executable()
	runner, err := configure.CreateRunner(repoRoot, hostBin, configure.Hooks{})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	errW := cmd.ErrOrStderr()

	cfg, err := config.Load(repoRoot, "")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "load config")
	}
	snap, err := configure.CollectSnapshot(ctx, repoRoot, cfg)
	if err != nil {
		return err
	}

	if wantJSON(cmd) && !flags.apply && flags.step == "" {
		return emitConfigureSnapshot(cmd, snap)
	}
	if flags.apply && flags.step != "" {
		return sirerr.New(sirerr.CodeInvalid, op, "use only one of --apply or --step")
	}
	if flags.apply {
		return runConfigureApply(cmd, ctx, runner, opts, tty, out, errW)
	}
	if flags.step != "" {
		return runConfigureStep(cmd, ctx, repoRoot, runner, flags.step, opts, out, errW)
	}
	if !tty {
		return sirerr.New(sirerr.CodeInvalid, op, "non-interactive: use --json, --apply, or --step")
	}
	_, err = configure.RunHub(ctx, runner, configure.HuhHubPrompter{}, opts, out, errW)
	return err
}

func emitConfigureSnapshot(cmd *cobra.Command, snap configure.Snapshot) error {
	const op = "cli.configure"
	if err := printJSON(cmd.OutOrStdout(), snap); err != nil {
		return err
	}
	if !snap.Ready {
		return sirerr.New(sirerr.CodeFailed, op, "not ready")
	}
	return nil
}

func runConfigureApply(cmd *cobra.Command, ctx context.Context, runner *configure.Runner, opts configure.Options, tty bool, out, errW io.Writer) error {
	const op = "cli.configure"
	opts.Interactive = tty
	var confirm func(ctx context.Context) (bool, error)
	if tty {
		hp := configure.HuhHubPrompter{}
		confirm = hp.ConfirmBrowserLogin
	}
	session, err := runner.RunAllMissing(ctx, opts, out, errW, confirm)
	return emitConfigureSession(cmd, out, session, err, op)
}

func runConfigureStep(cmd *cobra.Command, ctx context.Context, repoRoot string, runner *configure.Runner, stepID string, opts configure.Options, out, errW io.Writer) error {
	const op = "cli.configure"
	stepRes, _, err := runner.RunStep(ctx, stepID, opts, out, errW, true)
	cfg, loadErr := config.Load(repoRoot, "")
	if loadErr != nil && err == nil {
		return sirerr.Wrap(loadErr, sirerr.CodeFailed, op, "reload config after step")
	}
	snap, snapErr := configure.CollectSnapshot(ctx, repoRoot, cfg)
	if snapErr != nil && err == nil {
		return sirerr.Wrap(snapErr, sirerr.CodeFailed, op, "collect snapshot after step")
	}
	session := configure.SessionResult{
		Snapshot: snap,
		Ran:      []configure.StepResult{stepRes},
	}
	return emitConfigureSession(cmd, out, session, err, op)
}

func emitConfigureSession(cmd *cobra.Command, out io.Writer, session configure.SessionResult, err error, op string) error {
	if wantJSON(cmd) {
		if encErr := printJSON(out, session); encErr != nil {
			return encErr
		}
		return err
	}
	if wErr := writeCLILine(out, formatConfigureSession(session)); wErr != nil && err == nil {
		return sirerr.Wrap(wErr, sirerr.CodeFailed, op, "write configure session")
	}
	return err
}
