package cli

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func newMailOutcomesCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outcomes",
		Short: "Operator message outcome labels",
	}
	cmd.AddCommand(newMailOutcomesExportCmd(opts, repoRoot))
	return cmd
}

func newMailOutcomesExportCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Write message_outcomes rows to CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			const op = "cli.mail.outcomes.export"
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				return sirerr.New(sirerr.CodeInvalid, op, "nil context")
			}
			opCtx, cancel := contextWithStoreDeadline(ctx)
			defer cancel()
			st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: cfg.StorePath, Now: time.Now().UTC()})
			if err != nil {
				return wrapCLIHop(err, op, "open store")
			}
			defer func() { _ = st.Close() }()
			rows, err := st.ListOutcomes(opCtx)
			if err != nil {
				return wrapCLIHop(err, op, "list outcomes")
			}
			out := strings.TrimSpace(outPath)
			if out == "" {
				out = filepath.Join(repoRoot, "tmp", "message_outcomes.csv")
			}
			if err := writeOutcomesCSV(out, rows); err != nil {
				return wrapCLIHop(err, op, "write csv")
			}
			if wantJSON(cmd) {
				return printJSON(cmd.OutOrStdout(), map[string]any{"ok": true, "path": out, "count": len(rows)})
			}
			return writeCLILine(cmd.OutOrStdout(), fmt.Sprintf("Wrote %s (%d rows)\n", out, len(rows)))
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output CSV path (default tmp/message_outcomes.csv)")
	return cmd
}

func writeOutcomesCSV(path string, rows []hoststore.MessageOutcome) error {
	const op = "cli.writeOutcomesCSV"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir")
	}
	f, err := os.Create(path) //nolint:gosec // G304: operator-supplied outcomes CSV path.
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "create file")
	}
	defer func() { _ = f.Close() }()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"object_hash", "outcome", "source", "job_id", "sender", "decided_at"}); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write header")
	}
	for _, row := range rows {
		if err := w.Write([]string{
			row.ObjectHash, row.Outcome, row.Source, row.JobID, row.Sender, row.DecidedAt,
		}); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "write row")
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "flush csv")
	}
	return nil
}

func contextWithStoreDeadline(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if _, ok := parent.Deadline(); ok {
		return parent, func() {}
	}
	return context.WithTimeout(parent, hostStoreOpTimeout)
}
