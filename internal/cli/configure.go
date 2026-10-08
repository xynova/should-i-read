package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/configure"
	"github.com/xynova/should-i-read/internal/config"
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
		Use:   "configure",
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
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing mail-sync.toml")
	cmd.Flags().BoolVar(&skipLogin, "skip-login", false, "Stop before mailbox login on --apply")
	return cmd
}

type configureFlags struct {
	apply, force, skipLogin                bool
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

	cfg, _ := config.Load(repoRoot, "")
	snap, err := configure.CollectSnapshot(ctx, repoRoot, cfg)
	if err != nil {
		return err
	}

	if wantJSON(cmd) && !flags.apply && flags.step == "" {
		if err := printJSON(out, snap); err != nil {
			return err
		}
		if !snap.Ready {
			return sirerr.New(sirerr.CodeFailed, op, "not ready")
		}
		return nil
	}

	if flags.apply && flags.step != "" {
		return sirerr.New(sirerr.CodeInvalid, op, "use only one of --apply or --step")
	}

	if flags.apply {
		opts.Interactive = tty
		var confirm func(ctx context.Context) (bool, error)
		if tty {
			hp := configure.HuhHubPrompter{}
			confirm = hp.ConfirmBrowserLogin
		}
		session, err := runner.RunAllMissing(ctx, opts, out, errW, confirm)
		if wantJSON(cmd) {
			if encErr := printJSON(out, session); encErr != nil {
				return encErr
			}
			return err
		}
		fmt.Fprintln(out, formatConfigureSession(session))
		return err
	}

	if flags.step != "" {
		stepRes, _, err := runner.RunStep(ctx, flags.step, opts, out, errW, true)
		cfg, _ = config.Load(repoRoot, "")
		snap, _ = configure.CollectSnapshot(ctx, repoRoot, cfg)
		session := configure.SessionResult{
			Snapshot: snap,
			Ran:      []configure.StepResult{stepRes},
		}
		if wantJSON(cmd) {
			if encErr := printJSON(out, session); encErr != nil {
				return encErr
			}
			return err
		}
		fmt.Fprintln(out, formatConfigureSession(session))
		return err
	}

	if !tty {
		return sirerr.New(sirerr.CodeInvalid, op, "non-interactive: use --json, --apply, or --step")
	}

	_, err = configure.RunHub(ctx, runner, configure.HuhHubPrompter{}, opts, out, errW)
	return err
}
