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

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func newMailReadinessCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	return &cobra.Command{
		Use:   "readiness",
		Short: "Check mail sync readiness (credentials and IMAP)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailReadiness(cmd, opts, repoRoot)
		},
	}
}

func runMailReadiness(cmd *cobra.Command, opts *rootOptions, repoRoot string) error {
	ctx := cmd.Context()
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	nev, err := pimalaya.CreateNeverest(cfg)
	if err != nil {
		return err
	}
	acct := cfg.Pimalaya.DefaultAccount
	res, err := nev.Check(ctx)
	return finishPimalaya(cmd, opts, res, err, acct)
}

func newMailSyncCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync mail into the local store",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailSync(cmd, opts, repoRoot, account)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Mail account id from sync config")
	return cmd
}

func runMailSync(cmd *cobra.Command, opts *rootOptions, repoRoot string, account string) error {
	ctx := cmd.Context()
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	acct := account
	if acct == "" {
		acct = cfg.Pimalaya.DefaultAccount
	}
	nev, err := pimalaya.CreateNeverest(cfg)
	if err != nil {
		return err
	}
	res, err := nev.Sync(ctx, acct)
	return finishPimalaya(cmd, opts, res, err, acct)
}

func finishPimalaya(cmd *cobra.Command, opts *rootOptions, res *pimalaya.Result, err error, accountHint string) error {
	if res == nil {
		return err
	}
	if err != nil {
		printPimalayaFailure(cmd, opts, res, err, accountHint)
		if wantJSON(cmd) {
			_ = printPimalayaJSON(cmd.OutOrStdout(), res)
		}
		return err
	}
	return printPimalayaResult(cmd, res, accountHint)
}

func newMailExportCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		limit   int
		outPath string
		store   string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export recent mail summaries to JSON (report-only)",
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
	cmd.Flags().StringVar(&outPath, "out", "", "Output path (default tmp/mail-export-<ts>.json)")
	cmd.Flags().StringVar(&store, "store", "", "Local mail store directory (overrides config)")
	return cmd
}

type mailExportArtifact struct {
	Mode           string                `json:"mode"`
	ExportedAt     string                `json:"exported_at"`
	StoreDir       string                `json:"store_dir"`
	PolypusBaseURL string                `json:"polypus_base_url"`
	Count          int                   `json:"count"`
	Emails         []pimdir.EmailSummary `json:"emails"`
}

func runMailExport(cmd *cobra.Command, repoRoot, storeDir string, limit int, outPath, polypusURL string) error {
	const op = "cli.mail.export"
	reader, err := pimdir.OpenStore(storeDir)
	if err != nil {
		return err
	}
	emails, err := reader.ListRecentEmails(limit)
	if err != nil {
		return err
	}
	art := mailExportArtifact{
		Mode:           "report_only",
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
		StoreDir:       storeDir,
		PolypusBaseURL: polypusURL,
		Count:          len(emails),
		Emails:         emails,
	}
	out := outPath
	if out == "" {
		tmpDir := filepath.Join(repoRoot, "tmp")
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "create tmp dir")
		}
		out = filepath.Join(tmpDir, fmt.Sprintf("mail-export-%s.json", time.Now().UTC().Format("20060102T150405Z")))
	}
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode artifact")
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write artifact").With("path", out)
	}
	if wantJSON(cmd) {
		fmt.Fprintf(cmd.OutOrStdout(), "{\"ok\":true,\"path\":%q,\"count\":%d}\n", out, art.Count)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", clui.FormatBox("Export", fmt.Sprintf("Exported %d messages\n%s", art.Count, out)))
	return nil
}

func newMailShowCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		store   string
		raw     bool
		maxBody int
	)
	cmd := &cobra.Command{
		Use:   "show <ref>",
		Short: "Show one synced message body (object_hash or Message-ID from export JSON)",
		Args:  cobra.ExactArgs(1),
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
	cmd.Flags().StringVar(&store, "store", "", "Local mail store directory (overrides config)")
	cmd.Flags().BoolVar(&raw, "raw", false, "Write full RFC822 blob bytes to stdout")
	cmd.Flags().IntVar(&maxBody, "max-body", 8192, "Max body bytes in default preview mode")
	return cmd
}

func runMailShow(ctx context.Context, out io.Writer, storeDir, ref string, rawOut bool, maxBody int) error {
	const op = "cli.mail.show"
	_ = ctx
	if out == nil {
		out = os.Stdout
	}
	reader, err := pimdir.OpenStore(storeDir)
	if err != nil {
		return err
	}
	summary, hash, err := reader.ResolveShowRef(ref)
	if err != nil {
		return err
	}
	blob, err := reader.ReadBlob(hash)
	if err != nil {
		return err
	}
	if rawOut {
		_, werr := out.Write(blob)
		if werr != nil {
			return sirerr.Wrap(werr, sirerr.CodeFailed, op, "write raw body")
		}
		return nil
	}
	printMailShowHeader(out, summary)
	prev, err := pimdir.DecodeMessagePreview(blob, maxBody)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "decode message preview")
	}
	if _, err := fmt.Fprintln(out, prev.Body); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write body")
	}
	if prev.Truncated {
		fmt.Fprintf(out, "\n--- body truncated at %d bytes; use --raw for full RFC822 ---\n", maxBody)
	}
	return nil
}

func printMailShowHeader(w io.Writer, s pimdir.EmailSummary) {
	if w == nil {
		return
	}
	if s.Subject != "" {
		fmt.Fprintf(w, "%s %s\n", clui.Label("Subject:"), s.Subject)
	}
	from := strings.TrimSpace(s.SenderName)
	if from == "" {
		from = strings.TrimSpace(s.Sender)
	} else if strings.TrimSpace(s.Sender) != "" {
		from = from + " <" + strings.TrimSpace(s.Sender) + ">"
	}
	if from != "" {
		fmt.Fprintf(w, "%s %s\n", clui.Label("From:"), from)
	}
	if s.Date != "" {
		fmt.Fprintf(w, "%s %s\n", clui.Label("Date:"), s.Date)
	}
	if s.Collection != "" {
		fmt.Fprintf(w, "%s %s\n", clui.Label("Collection:"), s.Collection)
	}
	if s.MessageID != "" {
		fmt.Fprintf(w, "%s %s\n", clui.Label("Message-ID:"), s.MessageID)
	}
	if s.ObjectHash != "" {
		fmt.Fprintf(w, "%s %s\n", clui.Label("Object-Hash:"), s.ObjectHash)
	}
	fmt.Fprintln(w)
}
