package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailoverview"
	"github.com/xynova/should-i-read/internal/mailoverview/render"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type mailOverviewFlags struct {
	outPath string
	store   string
	since   string
	until   string
	maxRows int
	ledger  string
}

func bindMailOverviewFlags(cmd *cobra.Command, f *mailOverviewFlags) {
	cmd.Flags().StringVar(&f.outPath, "out", "", "Output path stem (default tmp/mail-overview-<ts>; writes .html and .json)")
	cmd.Flags().StringVar(&f.store, "store", "", "Local mail store directory (overrides config pimdir path)")
	cmd.Flags().StringVar(&f.since, "since", "", "First mail month YYYY-MM (inclusive)")
	cmd.Flags().StringVar(&f.until, "until", "", "Last mail month YYYY-MM (inclusive)")
	cmd.Flags().IntVar(&f.maxRows, "max-rows", 2000, "Max message rows per month in HTML (JSON keeps all)")
	cmd.Flags().StringVar(&f.ledger, "ledger", "", "Classify ledger path (overrides config store path)")
}

func newMailOverviewCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var f mailOverviewFlags
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Write offline mail overview HTML and JSON (read-only stores)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			return runMailOverview(cmd, repoRoot, cfg, f)
		},
	}
	bindMailOverviewFlags(cmd, &f)
	return cmd
}

type mailOverviewJSON struct {
	OK     bool   `json:"ok"`
	HTML   string `json:"html"`
	JSON   string `json:"json"`
	Months int    `json:"months"`
	Count  int    `json:"count"`
}

type overviewPage struct {
	HTML     string
	JSON     []byte
	Model    mailoverview.Model
	HTMLPath string
	JSONPath string
}

func runMailOverview(cmd *cobra.Command, repoRoot string, cfg config.Config, f mailOverviewFlags) error {
	page, err := buildOverviewPage(cmd.Context(), repoRoot, cfg, f)
	if err != nil {
		return err
	}
	if err := writeOverviewFiles(repoRoot, f.outPath, &page); err != nil {
		return err
	}

	if wantJSON(cmd) {
		out := mailOverviewJSON{
			OK:     true,
			HTML:   page.HTMLPath,
			JSON:   page.JSONPath,
			Months: len(page.Model.Months),
			Count:  page.Model.Provenance.CompleteRows,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	box := clui.FormatBox("Mail overview", fmt.Sprintf("Wrote\n%s\n%s\nrows %d months %d",
		page.HTMLPath, page.JSONPath, page.Model.Provenance.CompleteRows, len(page.Model.Months)))
	return writeCLI(cmd.OutOrStdout(), "%s\n", box)
}

func buildOverviewPage(parent context.Context, repoRoot string, cfg config.Config, f mailOverviewFlags) (overviewPage, error) {
	const op = "cli.mail.overview.build"
	if parent == nil {
		return overviewPage{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	ctx := parent
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, hostStoreOpTimeout)
		defer cancel()
	}
	ledger := strings.TrimSpace(f.ledger)
	if ledger == "" {
		ledger = cfg.StorePath
	}
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: ledger, Now: time.Now().UTC()})
	if err != nil {
		return overviewPage{}, wrapCLIHop(err, op, "open ledger")
	}
	defer func() { _ = st.Close() }()

	rows, err := st.ListCompleteOverviewRows(ctx)
	if err != nil {
		return overviewPage{}, wrapCLIHop(err, op, "list complete rows")
	}
	shapes, err := st.ListMailShapes(ctx)
	if err != nil {
		return overviewPage{}, wrapCLIHop(err, op, "list mail shapes")
	}
	errRows, err := st.ListErrorRows(ctx)
	if err != nil {
		return overviewPage{}, wrapCLIHop(err, op, "list error rows")
	}

	mailDir := f.store
	if mailDir == "" {
		mailDir = cfg.Pimalaya.PimdirPath
	}
	mailReader, err := pimdir.OpenStore(mailDir)
	if err != nil {
		return overviewPage{}, wrapCLIHop(err, op, "open pimdir")
	}

	summaries, err := mailReader.SummariesByObjectHash(ctx, hashList(rows))
	if err != nil {
		return overviewPage{}, wrapCLIHop(err, op, "load mail summaries")
	}

	gitRev := gitRevision(repoRoot)
	model := mailoverview.BuildModel(
		time.Now(),
		time.Local,
		mailoverview.Input{SinceMonth: f.since, UntilMonth: f.until, MaxTable: 0},
		rows,
		summaries,
		ledger,
		mailDir,
		Version,
		gitRev,
		shapes,
		errRows,
	)
	jsonBytes, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return overviewPage{}, wrapCLIHop(err, op, "marshal json")
	}
	mailoverview.CapMessageTables(&model, f.maxRows)
	htmlStr, err := render.RenderHTML(model)
	if err != nil {
		return overviewPage{}, wrapCLIHop(err, op, "render html")
	}
	return overviewPage{HTML: htmlStr, JSON: jsonBytes, Model: model}, nil
}

func writeOverviewFiles(repoRoot, outPath string, page *overviewPage) error {
	const op = "cli.mail.overview.write"
	if page == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil overview page")
	}
	stem := outPath
	if stem == "" {
		tmpDir := filepath.Join(repoRoot, "tmp")
		if err := os.MkdirAll(tmpDir, 0o750); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "create tmp dir")
		}
		stem = filepath.Join(tmpDir, fmt.Sprintf("mail-overview-%s", time.Now().UTC().Format("20060102T150405Z")))
	} else {
		stem = strings.TrimSuffix(stem, ".html")
		stem = strings.TrimSuffix(stem, ".json")
	}
	page.HTMLPath = stem + ".html"
	page.JSONPath = stem + ".json"
	if err := os.WriteFile(page.HTMLPath, []byte(page.HTML), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write html").With("path", page.HTMLPath)
	}
	if err := os.WriteFile(page.JSONPath, append(page.JSON, '\n'), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write json").With("path", page.JSONPath)
	}
	return nil
}

func hashList(rows []hoststore.OverviewRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		h := strings.TrimSpace(r.ObjectHash)
		if h != "" {
			out = append(out, h)
		}
	}
	return out
}

func gitRevision(repoRoot string) string {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
