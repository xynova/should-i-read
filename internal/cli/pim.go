package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
	"github.com/xynova/should-i-read/internal/triage"
)

func newPimCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pim",
		Short: "Pimalaya lane (Neverest + pimdir snapshot)",
	}
	cmd.AddCommand(newPimDoctorCmd(opts, repoRoot))
	cmd.AddCommand(newPimSyncCmd(opts, repoRoot))
	cmd.AddCommand(newPimSnapshotCmd(opts, repoRoot))
	cmd.AddCommand(newPimDigestCheckCmd(opts, repoRoot))
	return cmd
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
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(art)
		},
	}
}

func newPimDoctorCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run neverest check --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			nev, err := pimalaya.CreateNeverest(cfg)
			if err != nil {
				return err
			}
			res, err := nev.Check(cmd.Context())
			if res != nil {
				_ = printPimalayaResult(res)
			}
			return err
		},
	}
}

func newPimSyncCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run neverest sync --json",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			res, err := nev.Sync(cmd.Context(), acct)
			if res != nil {
				_ = printPimalayaResult(res)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Neverest account id")
	return cmd
}

func newPimSnapshotCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		limit   int
		outPath string
		store   string
	)
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Export recent mail summaries from pimdir SQLite (report-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			dir := store
			if dir == "" {
				dir = cfg.Pimalaya.PimdirPath
			}
			return runPimSnapshot(cmd.Context(), repoRoot, dir, limit, outPath, cfg.PolypusBaseURL)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max messages")
	cmd.Flags().StringVar(&outPath, "out", "", "Output path (default tmp/pim-snapshot-<ts>.json)")
	cmd.Flags().StringVar(&store, "store", "", "Pimdir store directory (overrides config)")
	return cmd
}

type pimSnapshotArtifact struct {
	Mode           string              `json:"mode"`
	ExportedAt     string              `json:"exported_at"`
	StoreDir       string              `json:"store_dir"`
	PolypusBaseURL string              `json:"polypus_base_url"`
	Count          int                 `json:"count"`
	Emails         []pimdir.EmailSummary `json:"emails"`
}

func runPimSnapshot(ctx context.Context, repoRoot, storeDir string, limit int, outPath, polypusURL string) error {
	const op = "cli.pim.snapshot"
	_ = ctx
	reader, err := pimdir.OpenStore(storeDir)
	if err != nil {
		return err
	}
	emails, err := reader.ListRecentEmails(limit)
	if err != nil {
		return err
	}
	art := pimSnapshotArtifact{
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
		out = filepath.Join(tmpDir, fmt.Sprintf("pim-snapshot-%s.json", time.Now().UTC().Format("20060102T150405Z")))
	}
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode artifact")
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write artifact").With("path", out)
	}
	fmt.Fprintf(os.Stdout, "{\"ok\":true,\"path\":%q,\"count\":%d}\n", out, art.Count)
	return nil
}

func printPimalayaResult(res *pimalaya.Result) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if len(res.Raw) > 0 {
		return enc.Encode(json.RawMessage(res.Raw))
	}
	return enc.Encode(map[string]any{
		"ok":     res.OK,
		"exit":   res.Exit,
		"stderr": res.Stderr,
	})
}
