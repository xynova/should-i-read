package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/emailops"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Version is set via -ldflags at build time when available.
var Version = "dev"

type rootOptions struct {
	dataDir string
	quiet   bool
	jsonOut bool
}

// Execute runs the root command.
func Execute(ctx context.Context, repoRoot string) int {
	opts := &rootOptions{jsonOut: true}
	cfg := config.Create(repoRoot)

	root := &cobra.Command{
		Use:           "should-i-read",
		Short:         "Host operator CLI for EmailOps (black box) and Polypus",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return sirerr.New(sirerr.CodeInvalid, "cli.root", "command required")
		},
	}
	root.PersistentFlags().StringVar(&opts.dataDir, "data-dir", "", "EmailOps data directory (default: EMAILOPS_DATA_DIR or platform path)")
	root.PersistentFlags().BoolVar(&opts.quiet, "quiet", false, "Pass --quiet to emailops-cli")
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	root.AddCommand(newVersionCmd())
	root.AddCommand(newDoctorCmd(cfg, opts))
	root.AddCommand(newAccountsCmd(cfg, opts))
	root.AddCommand(newSyncCmd(cfg, opts))
	root.AddCommand(newEmailsCmd(cfg, opts))
	root.AddCommand(newShowCmd(cfg, opts))
	root.AddCommand(newExportCmd(cfg, opts, repoRoot))
	root.AddCommand(newPolypusCmd(cfg))

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return sirerr.ExitCode(err)
	}
	return 0
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
	return client, nil
}

func printEnvelope(env *emailops.Envelope) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(env)
}

func newDoctorCmd(cfg config.Config, opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run emailops-cli doctor --json",
		RunE: func(cmd *cobra.Command, args []string) error {
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

func newAccountsCmd(cfg config.Config, opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "accounts",
		Short: "List EmailOps accounts (--json)",
		RunE: func(cmd *cobra.Command, args []string) error {
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

func newSyncCmd(cfg config.Config, opts *rootOptions) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync mail via emailops-cli (app closed recommended)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newEmailOpsClient(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			cliArgs := []string{"sync"}
			if account != "" {
				cliArgs = append(cliArgs, account)
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

func newEmailsCmd(cfg config.Config, opts *rootOptions) *cobra.Command {
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
			client, err := newEmailOpsClient(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			cliArgs := []string{"emails", "--limit", fmt.Sprintf("%d", limit)}
			if offset > 0 {
				cliArgs = append(cliArgs, "--offset", fmt.Sprintf("%d", offset))
			}
			if mailbox != "" {
				cliArgs = append(cliArgs, "--mailbox", mailbox)
			}
			if account != "" {
				cliArgs = append(cliArgs, "--account", account)
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

func newShowCmd(cfg config.Config, opts *rootOptions) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one email via emailops-cli --json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newEmailOpsClient(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			cliArgs := []string{"show", args[0]}
			if account != "" {
				cliArgs = append(cliArgs, "--account", account)
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

func newExportCmd(cfg config.Config, opts *rootOptions, repoRoot string) *cobra.Command {
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
			return runExport(cmd.Context(), cfg, opts, repoRoot, exportOptions{
				Account:    account,
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

func newPolypusCmd(cfg config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "polypus",
		Short: "Polypus gateway helpers",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Fail-closed /health and /v1/models probe",
		RunE: func(cmd *cobra.Command, args []string) error {
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
