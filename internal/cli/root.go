package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/emailops"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/setup"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Version is set via -ldflags at build time when available.
var Version = "dev"

type rootOptions struct {
	configPath string
	dataDir    string
	quiet      bool
	cfg        config.Config
	loaded     bool
}

// Execute runs the root command.
func Execute(ctx context.Context, repoRoot string) int {
	opts := &rootOptions{}

	root := &cobra.Command{
		Use:           "should-i-read",
		Short:         "Host operator CLI for EmailOps (black box) and Polypus",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return startTelemetry(cmd.Root().Name())
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return stopTelemetry()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return sirerr.New(sirerr.CodeInvalid, "cli.root", "command required")
		},
	}
	root.PersistentFlags().StringVar(&opts.configPath, "config", "", "Config file (default: SHOULD_I_READ_CONFIG or ~/.config/should-i-read/config.yaml)")
	root.PersistentFlags().StringVar(&opts.dataDir, "data-dir", "", "EmailOps data directory (overrides config)")
	root.PersistentFlags().BoolVar(&opts.quiet, "quiet", false, "Pass --quiet to emailops-cli")
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd(repoRoot))
	root.AddCommand(newSetupCmd(repoRoot))
	root.AddCommand(newConfigCmd(opts, repoRoot))
	root.AddCommand(newSecretCmd())
	root.AddCommand(newDoctorCmd(opts, repoRoot))
	root.AddCommand(newAccountsCmd(opts, repoRoot))
	root.AddCommand(newSyncCmd(opts, repoRoot))
	root.AddCommand(newEmailsCmd(opts, repoRoot))
	root.AddCommand(newShowCmd(opts, repoRoot))
	root.AddCommand(newExportCmd(opts, repoRoot))
	root.AddCommand(newPolypusCmd(opts, repoRoot))
	root.AddCommand(newPimCmd(opts, repoRoot))
	root.AddCommand(newTokenCmd(opts, repoRoot))
	root.AddCommand(newUICmd(opts, repoRoot))

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return sirerr.ExitCode(err)
	}
	return 0
}

func (o *rootOptions) load(repoRoot string) error {
	if o.loaded {
		return nil
	}
	cfg, err := config.Load(repoRoot, o.configPath)
	if err != nil {
		return err
	}
	o.cfg = cfg
	o.loaded = true
	return nil
}

func (o *rootOptions) mustLoad(repoRoot string) (config.Config, error) {
	if err := o.load(repoRoot); err != nil {
		return config.Config{}, err
	}
	return o.cfg, nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
		},
	}
}

func resolveDataDir(cfg config.Config, opts *rootOptions) string {
	if opts != nil && opts.dataDir != "" {
		return opts.dataDir
	}
	return cfg.EmailOpsDataDir
}

func newEmailOpsClient(ctx context.Context, cfg config.Config, opts *rootOptions) (*emailops.Client, error) {
	bin, err := emailops.EnsureBin(ctx, cfg.EmailOpsCLI, cfg.RepoRoot)
	if err != nil {
		return nil, err
	}
	client, err := emailops.Create(bin, resolveDataDir(cfg, opts))
	if err != nil {
		return nil, err
	}
	client.Quiet = opts.quiet
	client.ExtraEnv = cfg.ChildEnv()
	return client, nil
}

func printEnvelope(env *emailops.Envelope) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(env)
}

func newInitCmd(repoRoot string) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create ~/.config/should-i-read/config.yaml and default data dir",
		RunE: func(cmd *cobra.Command, args []string) error {
			const op = "cli.init"
			examplePath := filepath.Join(repoRoot, "config", "should-i-read.example.yaml")
			var exampleSrc []byte
			if raw, err := os.ReadFile(examplePath); err == nil {
				exampleSrc = raw
			}
			path, created, err := config.WriteInit(force, exampleSrc)
			if err != nil {
				return err
			}
			dataDir := config.DefaultEmailOpsDataDir()
			if dataDir != "" {
				if err := os.MkdirAll(dataDir, 0o700); err != nil {
					return sirerr.Wrap(err, sirerr.CodeFailed, op, "create default data dir").With("dir", dataDir)
				}
			}
			status := "unchanged"
			if created {
				status = "created"
			} else if force {
				status = "replaced"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "{\"ok\":true,\"config\":%q,\"status\":%q,\"data_dir\":%q}\n", path, status, dataDir)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config.yaml")
	return cmd
}

func newSetupCmd(repoRoot string) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive mail OAuth setup (product creds, BYO, or guided DIY)",
		Long: `Interactive installer for EmailOps OAuth *client* credentials.

Product-owned client ids are the default when present (env, Keychain, or release
embed). Otherwise choose BYO paste or advanced guided Cloud Console / Entra DIY.

Mailbox tokens stay in EmailOps on this machine. Client secrets are never printed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			const op = "cli.setup"
			// Caller deadline for optional gcloud / process work.
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()

			wiz, err := setup.Create(setup.Config{
				RepoRoot: repoRoot,
				Out:      cmd.OutOrStdout(),
				Err:      cmd.ErrOrStderr(),
				Prompter: setup.HuhPrompter{},
			})
			if err != nil {
				return sirerr.Wrap(err, sirerr.CodeFailed, op, "create setup wizard")
			}
			res, err := wiz.Run(ctx)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if encErr := enc.Encode(res); encErr != nil {
				return sirerr.Wrap(encErr, sirerr.CodeFailed, op, "encode setup result")
			}
			return nil
		},
	}
}

func newConfigCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect operator config",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print resolved config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(opts.configPath)
			if err != nil {
				return err
			}
			if path == "" {
				userPath, uerr := config.UserConfigFilePath()
				if uerr != nil {
					return uerr
				}
				fmt.Fprintln(cmd.OutOrStdout(), userPath+" (missing; run should-i-read init)")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print redacted effective config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(cfg.Redacted())
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "bump",
		Short: "Patch live config toward the current template (non-destructive)",
		Long: `Updates ~/.config/should-i-read/config.yaml in place when safe:
adds POLYPUS_BASE_URL to secrets: and replaces a literal http://127.0.0.1:1320
polypus.base_url with ${POLYPUS_BASE_URL}. Does not overwrite custom Polypus URLs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, changed, notes, err := config.BumpOperatorConfig()
			if err != nil {
				return err
			}
			out := map[string]any{
				"ok":      true,
				"config":  path,
				"changed": changed,
				"notes":   notes,
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	})
	return cmd
}

func newSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage platform keyring secrets (operatorconfig)",
	}
	var stdin bool
	setCmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Store a secret in the platform keyring (service should-i-read)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			var value string
			if stdin {
				raw, err := io.ReadAll(os.Stdin)
				if err != nil {
					return sirerr.Wrap(err, sirerr.CodeFailed, "cli.secret.set", "read stdin")
				}
				value = strings.TrimSpace(string(raw))
			} else {
				fmt.Fprint(os.Stderr, "value: ")
				raw, err := io.ReadAll(io.LimitReader(os.Stdin, 64*1024))
				if err != nil {
					return sirerr.Wrap(err, sirerr.CodeFailed, "cli.secret.set", "read value")
				}
				value = strings.TrimSpace(string(raw))
			}
			if err := config.SetSecret(name, value); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "{\"ok\":true,\"name\":%q}\n", name)
			return nil
		},
	}
	setCmd.Flags().BoolVar(&stdin, "stdin", false, "Read secret value from stdin")
	cmd.AddCommand(setCmd)
	cmd.AddCommand(&cobra.Command{
		Use:   "delete <name>",
		Short: "Remove a keyring secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.DeleteSecret(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "{\"ok\":true,\"deleted\":%q}\n", args[0])
			return nil
		},
	})
	return cmd
}

func newUICmd(opts *rootOptions, repoRoot string) *cobra.Command {
	return &cobra.Command{
		Use:   "ui",
		Short: "Launch EmailOps desktop with config data dir and OAuth env",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			extra := cfg.ChildEnv()
			if opts.dataDir != "" {
				filtered := make([]string, 0, len(extra)+1)
				for _, kv := range extra {
					if strings.HasPrefix(kv, "EMAILOPS_DATA_DIR=") {
						continue
					}
					filtered = append(filtered, kv)
				}
				filtered = append(filtered, "EMAILOPS_DATA_DIR="+opts.dataDir)
				extra = filtered
			}
			return emailops.LaunchUI(cmd.Context(), cfg.EmailOpsRepoPath, extra)
		},
	}
}

func newDoctorCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run emailops-cli doctor --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			client, err := newEmailOpsClient(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			env, err := client.Run(cmd.Context(), "doctor")
			if env != nil {
				_ = printEnvelope(env)
			}
			return err
		},
	}
}

func newAccountsCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	return &cobra.Command{
		Use:   "accounts",
		Short: "List EmailOps accounts (--json)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			client, err := newEmailOpsClient(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			env, err := client.Run(cmd.Context(), "accounts")
			if env != nil {
				_ = printEnvelope(env)
			}
			return err
		},
	}
}

func newSyncCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync mail via emailops-cli (app closed recommended)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			client, err := newEmailOpsClient(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			acct := account
			if acct == "" {
				acct = cfg.DefaultAccount
			}
			cliArgs := []string{"sync"}
			if acct != "" {
				cliArgs = append(cliArgs, acct)
			}
			env, err := client.Run(cmd.Context(), cliArgs...)
			if env != nil {
				_ = printEnvelope(env)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Account id or email")
	return cmd
}

func newEmailsCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		account string
		limit   int
		mailbox string
		offset  int
	)
	cmd := &cobra.Command{
		Use:   "emails",
		Short: "List recent emails via emailops-cli --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			client, err := newEmailOpsClient(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			acct := account
			if acct == "" {
				acct = cfg.DefaultAccount
			}
			cliArgs := []string{"emails", "--limit", fmt.Sprintf("%d", limit)}
			if offset > 0 {
				cliArgs = append(cliArgs, "--offset", fmt.Sprintf("%d", offset))
			}
			if mailbox != "" {
				cliArgs = append(cliArgs, "--mailbox", mailbox)
			}
			if acct != "" {
				cliArgs = append(cliArgs, "--account", acct)
			}
			env, err := client.Run(cmd.Context(), cliArgs...)
			if env != nil {
				_ = printEnvelope(env)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Account id or email")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max emails to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip this many emails")
	cmd.Flags().StringVar(&mailbox, "mailbox", "", "Mailbox: inbox|sent|spam|trash")
	return cmd
}

func newShowCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one email via emailops-cli --json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			client, err := newEmailOpsClient(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			acct := account
			if acct == "" {
				acct = cfg.DefaultAccount
			}
			cliArgs := []string{"show", args[0]}
			if acct != "" {
				cliArgs = append(cliArgs, "--account", acct)
			}
			env, err := client.Run(cmd.Context(), cliArgs...)
			if env != nil {
				_ = printEnvelope(env)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Account id or email")
	return cmd
}

func newExportCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		account    string
		limit      int
		mailbox    string
		withBodies bool
		outPath    string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export emails to tmp/mail-export-<ts>.json (report-only input)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			acct := account
			if acct == "" {
				acct = cfg.DefaultAccount
			}
			return runExport(cmd.Context(), cfg, opts, repoRoot, exportOptions{
				Account:    acct,
				Limit:      limit,
				Mailbox:    mailbox,
				WithBodies: withBodies,
				OutPath:    outPath,
			})
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Account id or email")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max emails to export")
	cmd.Flags().StringVar(&mailbox, "mailbox", "inbox", "Mailbox filter")
	cmd.Flags().BoolVar(&withBodies, "bodies", false, "Fetch full body via show for each message")
	cmd.Flags().StringVar(&outPath, "out", "", "Output path (default tmp/mail-export-<ts>.json)")
	return cmd
}

type exportOptions struct {
	Account    string
	Limit      int
	Mailbox    string
	WithBodies bool
	OutPath    string
}

type exportArtifact struct {
	Mode       string            `json:"mode"`
	ExportedAt string            `json:"exported_at"`
	DataDir    string            `json:"data_dir"`
	Account    string            `json:"account,omitempty"`
	Mailbox    string            `json:"mailbox,omitempty"`
	Count      int               `json:"count"`
	Emails     json.RawMessage   `json:"emails"`
	Bodies     []json.RawMessage `json:"bodies,omitempty"`
}

func runExport(ctx context.Context, cfg config.Config, opts *rootOptions, repoRoot string, eo exportOptions) error {
	const op = "cli.export"
	client, err := newEmailOpsClient(ctx, cfg, opts)
	if err != nil {
		return err
	}
	cliArgs := []string{"emails", "--limit", fmt.Sprintf("%d", eo.Limit)}
	if eo.Mailbox != "" {
		cliArgs = append(cliArgs, "--mailbox", eo.Mailbox)
	}
	if eo.Account != "" {
		cliArgs = append(cliArgs, "--account", eo.Account)
	}
	env, err := client.Run(ctx, cliArgs...)
	if err != nil {
		return err
	}

	art := exportArtifact{
		Mode:       "mail_export",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		DataDir:    resolveDataDir(cfg, opts),
		Account:    eo.Account,
		Mailbox:    eo.Mailbox,
		Emails:     env.Data,
	}

	if eo.WithBodies {
		ids, idErr := extractEmailIDs(env.Data)
		if idErr != nil {
			return sirerr.Wrap(idErr, sirerr.CodeFailed, op, "parse email ids for bodies")
		}
		bodies := make([]json.RawMessage, 0, len(ids))
		for _, id := range ids {
			showArgs := []string{"show", id}
			if eo.Account != "" {
				showArgs = append(showArgs, "--account", eo.Account)
			}
			showEnv, showErr := client.Run(ctx, showArgs...)
			if showErr != nil {
				return showErr
			}
			bodies = append(bodies, showEnv.Data)
		}
		art.Bodies = bodies
		art.Count = len(bodies)
	} else {
		art.Count = countJSONArray(env.Data)
	}

	out := eo.OutPath
	if out == "" {
		tmpDir := filepath.Join(repoRoot, "tmp")
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "create tmp dir")
		}
		out = filepath.Join(tmpDir, fmt.Sprintf("mail-export-%s.json", time.Now().UTC().Format("20060102T150405Z")))
	}
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode export artifact")
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write export artifact").With("path", out)
	}
	fmt.Fprintf(os.Stdout, "{\"ok\":true,\"path\":%q,\"count\":%d}\n", out, art.Count)
	return nil
}

func extractEmailIDs(data json.RawMessage) ([]string, error) {
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(arr))
	for _, row := range arr {
		for _, key := range []string{"id", "emailId", "email_id"} {
			if v, ok := row[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					ids = append(ids, s)
					break
				}
			}
		}
	}
	return ids, nil
}

func countJSONArray(data json.RawMessage) int {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return 0
	}
	return len(arr)
}

func newPolypusCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "polypus",
		Short: "Polypus gateway helpers",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Fail-closed /health and /v1/models probe",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			client, err := polypus.Create(cfg.PolypusBaseURL, nil)
			if err != nil {
				return err
			}
			result, err := client.Check(cmd.Context())
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	})
	return cmd
}
