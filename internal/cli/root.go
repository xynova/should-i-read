package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/setup"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Version is set via -ldflags at build time when available.
var Version = "dev"

type rootOptions struct {
	configPath string
	cfg        config.Config
	loaded     bool
	jsonOut    bool
	humanErr   bool
}

// skipTelemetryForCmd skips olly for commands that must stay fast and must not
// fail when no OTLP collector is reachable (shutdown flush uses a 5s cap).
func skipTelemetryForCmd(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case VerbConfigure:
			if isTerminal(os.Stdin) {
				return true
			}
		case "pim", "mail", "polypus":
			return true
		}
	}
	return false
}

// Execute runs the root command.
func Execute(ctx context.Context, repoRoot string) int {
	opts := &rootOptions{}

	root := &cobra.Command{
		Use:           "should-i-read",
		Short:         "Local inbox triage: mail sync, Polypus, report-only",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if skipTelemetryForCmd(cmd) {
				return nil
			}
			return startTelemetry(cmd.Root().Name())
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			_ = stopTelemetry()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return sirerr.New(sirerr.CodeInvalid, "cli.root", "command required")
		},
	}
	root.PersistentFlags().StringVar(&opts.configPath, "config", "", "Config file (default: SHOULD_I_READ_CONFIG or ~/.config/should-i-read/config.yaml)")
	root.PersistentFlags().BoolVar(&opts.jsonOut, "json", false, "Print machine JSON instead of a human summary")
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd(opts, repoRoot))
	root.AddCommand(newConfigureCmd(opts, repoRoot))
	root.AddCommand(newSetupCmd(opts, repoRoot))
	root.AddCommand(newConfigCmd(opts, repoRoot))
	root.AddCommand(newSecretCmd())
	root.AddCommand(newPolypusCmd(opts, repoRoot))
	root.AddCommand(newMailCmd(opts, repoRoot))
	root.AddCommand(newPimCmd(opts, repoRoot))
	root.AddCommand(newTokenCmd(opts, repoRoot))

	if err := root.ExecuteContext(ctx); err != nil {
		if !opts.humanErr {
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeCLILine(cmd.OutOrStdout(), Version)
		},
	}
}

func newInitCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   VerbInit,
		Short: "Create ~/.config/should-i-read/config.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			const op = "cli.init"
			examplePath := filepath.Join(repoRoot, "config", "should-i-read.example.yaml")
			var exampleSrc []byte
			//nolint:gosec // bundled example config under repoRoot
			if raw, err := os.ReadFile(examplePath); err == nil {
				exampleSrc = raw
			}
			path, created, err := config.WriteInit(force, exampleSrc)
			if err != nil {
				return sirerr.Wrap(err, sirerr.CodeFailed, op, "write init config")
			}
			status := "unchanged"
			if created {
				status = "created"
			} else if force {
				status = "replaced"
			}
			if wantJSON(cmd) {
				return writeCLI(cmd.OutOrStdout(), "{\"ok\":true,\"config\":%q,\"status\":%q}\n", path, status)
			}
			return writeCLI(cmd.OutOrStdout(), "%s %s\n", status, path)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config.yaml")
	return cmd
}

func newSetupCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "setup",
		Hidden:     true,
		Short:      "OAuth client setup only (deprecated: use configure)",
		Deprecated: "use should-i-read configure",
		Long: `Interactive installer for OAuth *client* credentials used by should-i-read
token gmail/outlook (Neverest XOAUTH2).

Prefer should-i-read configure for full operator onboarding.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			const op = "cli.setup"
			if err := writeCLIString(cmd.ErrOrStderr(), deprecationConfigure); err != nil {
				return sirerr.Wrap(err, sirerr.CodeFailed, op, "write deprecation")
			}
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
			if wantJSON(cmd) {
				return printJSON(cmd.OutOrStdout(), res)
			}
			return writeCLILine(cmd.OutOrStdout(), formatSetupResult(res))
		},
	}
	return cmd
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
				return writeCLILine(cmd.OutOrStdout(), userPath+" (missing; run should-i-read init)")
			}
			return writeCLILine(cmd.OutOrStdout(), path)
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
			if wantJSON(cmd) {
				return printJSON(cmd.OutOrStdout(), cfg.Redacted())
			}
			return writeCLILine(cmd.OutOrStdout(), formatConfigRedacted(cfg.Redacted()))
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
			if wantJSON(cmd) {
				return printJSON(cmd.OutOrStdout(), out)
			}
			return writeCLILine(cmd.OutOrStdout(), formatConfigBump(path, changed, notes))
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
				if err := writeCLIString(os.Stderr, "value: "); err != nil {
					return sirerr.Wrap(err, sirerr.CodeFailed, "cli.secret.set", "write prompt")
				}
				raw, err := io.ReadAll(io.LimitReader(os.Stdin, 64*1024))
				if err != nil {
					return sirerr.Wrap(err, sirerr.CodeFailed, "cli.secret.set", "read value")
				}
				value = strings.TrimSpace(string(raw))
			}
			if err := config.SetSecret(name, value); err != nil {
				return err
			}
			if wantJSON(cmd) {
				return writeCLI(cmd.OutOrStdout(), "{\"ok\":true,\"name\":%q}\n", name)
			}
			return writeCLI(cmd.OutOrStdout(), "Stored %s (keyring)\n", name)
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
			if wantJSON(cmd) {
				return writeCLI(cmd.OutOrStdout(), "{\"ok\":true,\"deleted\":%q}\n", args[0])
			}
			return writeCLI(cmd.OutOrStdout(), "Deleted %s (keyring)\n", args[0])
		},
	})
	return cmd
}

func newPolypusCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "polypus",
		Short: "Polypus gateway helpers",
	}
	var classifyCheck bool
	check := &cobra.Command{
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
			if classifyCheck {
				resolveCtx, cancel := context.WithTimeout(cmd.Context(), classifyModelResolveTimeout)
				defer cancel()
				judge, src, err := mailreport.ResolveJudgeModel(resolveCtx, client, result.ModelIDs, cfg.PolypusJudgeModel)
				if err != nil {
					return err
				}
				author, authorSrc, err := mailreport.ResolveAuthorModel(resolveCtx, client, result.ModelIDs, cfg.PolypusClassifyModel)
				if err != nil {
					return err
				}
				result.ClassifyReady = true
				result.JudgeModel = judge
				result.JudgeSource = string(src)
				result.AuthorModel = author
				result.AuthorSource = string(authorSrc)
			}
			if wantJSON(cmd) {
				return printJSON(cmd.OutOrStdout(), result)
			}
			return writeCLILine(cmd.OutOrStdout(), formatPolypusHealth(result, classifyCheck))
		},
	}
	check.Flags().BoolVar(&classifyCheck, "classify", false, "Smoke SystemOne judge and chat author for classify")
	cmd.AddCommand(check)
	return cmd
}
