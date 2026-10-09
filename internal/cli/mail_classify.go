package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/strop/pkg/embed"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type classifyOpts struct {
	catalogPath string
	noApply     bool
	maxBody     int
}

func newMailReportCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		inPath  string
		outPath string
		limit   int
		store   string
		catalog string
		noApply bool
		maxBody int
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Classify recent mail (report-only, Polypus + taxonomy)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailReport(cmd, opts, repoRoot, inPath, outPath, limit, store, classifyOpts{
				catalogPath: catalog,
				noApply:     noApply,
				maxBody:     maxBody,
			})
		},
	}
	cmd.Flags().StringVar(&inPath, "in", "", "Input mail export JSON (default: list from pimdir)")
	cmd.Flags().StringVar(&outPath, "out", "", "Report path (default tmp/unwanted-report-<ts>.json)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max messages when listing from pimdir")
	cmd.Flags().StringVar(&store, "store", "", "Local mail store directory (overrides config)")
	cmd.Flags().StringVar(&catalog, "catalog", "", "Taxonomy catalog YAML path")
	cmd.Flags().BoolVar(&noApply, "no-apply", false, "Do not write catalog mutations to disk")
	cmd.Flags().IntVar(&maxBody, "max-body", 8192, "Max body bytes per message preview")
	return cmd
}

func runMailReport(cmd *cobra.Command, opts *rootOptions, repoRoot string, inPath, outPath string, limit int, storeDir string, cOpts classifyOpts) error {
	const op = "cli.mail.report"
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	dir := storeDir
	if dir == "" {
		dir = cfg.Pimalaya.PimdirPath
	}
	reader, err := pimdir.OpenStore(dir)
	if err != nil {
		return err
	}
	items, err := loadReportItems(reader, inPath, limit, cOpts.maxBody, cfg.Taxonomy.Collections)
	if err != nil {
		return err
	}
	runner, model, err := prepareClassifyRunner(cmd.Context(), cfg, repoRoot, cOpts, true)
	if err != nil {
		return err
	}
	_ = model
	items = mailreport.DedupeItemsByHash(items)
	art, err := runner.Classify(cmd.Context(), mailreport.SourceReport, items)
	if err != nil {
		return err
	}
	path, err := writeReportArtifact(repoRoot, outPath, art)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write report")
	}
	art.ReportPath = path
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), art)
	}
	fmt.Fprintln(cmd.OutOrStdout(), formatClassifyArtifact(art))
	return nil
}

func runMailClassifyAfterSync(cmd *cobra.Command, opts *rootOptions, repoRoot string, res *pimalaya.Result, noClassify bool, maxBody int) error {
	const op = "cli.mail.classifyAfterSync"
	if noClassify || res == nil || len(res.Raw) == 0 {
		return nil
	}
	rep := pimalaya.SummarizeNeverestJSON(res.Raw)
	if !rep.Recognized || rep.DryRun || rep.Fetch == 0 {
		return nil
	}
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	reader, err := pimdir.OpenStore(cfg.Pimalaya.PimdirPath)
	if err != nil {
		return nil
	}
	items, unresolved := itemsFromFetchHunks(reader, pimalaya.FetchedHunks(res.Raw), maxBody)
	runner, _, err := prepareClassifyRunner(cmd.Context(), cfg, repoRoot, classifyOpts{maxBody: maxBody}, false)
	if err != nil {
		if isClassifySkip(err) {
			fmt.Fprintln(cmd.OutOrStdout(), clui.FormatBox("Classify", clui.Muted("Classification skipped (Polypus unavailable)")))
			return nil
		}
		return err
	}
	items = mailreport.DedupeItemsByHash(items)
	art, err := runner.Classify(cmd.Context(), mailreport.SourceSyncFetch, items)
	if err != nil {
		if isClassifySkip(err) {
			fmt.Fprintln(cmd.OutOrStdout(), clui.FormatBox("Classify", clui.Muted("Classification skipped (Polypus unavailable)")))
			return nil
		}
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "classify after sync")
	}
	art.Unresolved += unresolved
	path, werr := writeReportArtifact(repoRoot, "", art)
	if werr == nil {
		art.ReportPath = path
	}
	if wantJSON(cmd) {
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), formatClassifyArtifact(art))
	return nil
}

func prepareClassifyRunner(ctx context.Context, cfg config.Config, repoRoot string, cOpts classifyOpts, failClosed bool) (*mailreport.Runner, string, error) {
	const op = "cli.prepareClassifyRunner"
	if ctx == nil {
		return nil, "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	catalogPath := strings.TrimSpace(cOpts.catalogPath)
	if catalogPath == "" {
		catalogPath = cfg.Taxonomy.CatalogPath
	}
	seed := config.SeedKindCatalogPath(repoRoot)
	if err := mailreport.EnsureCatalog(catalogPath, seed); err != nil {
		return nil, "", err
	}
	cat, vocab, err := mailreport.LoadCatalog(catalogPath)
	if err != nil {
		return nil, "", err
	}
	client, err := polypus.Create(cfg.PolypusBaseURL, nil)
	if err != nil {
		return nil, "", err
	}
	health, err := client.Check(ctx)
	if err != nil {
		if failClosed {
			return nil, "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "polypus check")
		}
		return nil, "", classifySkipErr(err)
	}
	resolveCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	judge, _, err := mailreport.ResolveJudgeModel(resolveCtx, client, health.ModelIDs, cfg.PolypusJudgeModel)
	if err != nil {
		if failClosed {
			return nil, "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "resolve judge model")
		}
		return nil, "", classifySkipErr(err)
	}
	author, _, err := mailreport.ResolveAuthorModel(resolveCtx, client, health.ModelIDs, cfg.PolypusClassifyModel)
	if err != nil {
		if failClosed {
			return nil, "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "resolve author model")
		}
		return nil, "", classifySkipErr(err)
	}
	essenceModel := author
	embedFac := mailreport.NewStropEmbedFactory()
	embedModel, err := mailreport.ResolveEmbedModel(resolveCtx, embedFac, cfg.PolypusBaseURL, health.ModelIDs, cfg.PolypusEmbedModel)
	if err != nil {
		if failClosed {
			return nil, "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "resolve embed model")
		}
		return nil, "", classifySkipErr(err)
	}
	stropEmb, err := embedFac.CreateEmbedder(resolveCtx, cfg.PolypusBaseURL, embedModel)
	if err != nil {
		if failClosed {
			return nil, "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "create embedder")
		}
		return nil, "", classifySkipErr(err)
	}
	embedSeat := mailreport.NewStropEmbedSeat(stropEmb, embedModel)
	seats, err := mailreport.CreateSeats(mailreport.SeatsConfig{
		Client:       client,
		JudgeModel:   judge,
		AuthorModel:  author,
		EssenceModel: essenceModel,
		Embedder:     embedSeat,
	})
	if err != nil {
		return nil, "", err
	}
	authorSeat := seats.Author
	if !cfg.Taxonomy.AuthorOnSkip {
		authorSeat = mailreport.NoopAuthor()
	}
	if os.Getenv("SHOULD_I_READ_TRY_MIN_JUDGE") == "0.80" {
		authorSeat = seats.Author
	}
	hCfg := harness.Config{
		Judge:            seats.Judge,
		Author:           authorSeat,
		MinJudgeScore:    tryMinJudgeScore(),
		Strategy:         harness.StrategyAttach,
		Embedder:         seats.Embedder,
		Essencer:         seats.Essencer,
		AttachMinCosine:  cfg.Taxonomy.AttachMinCosine,
		WalkReinforceMin: cfg.Taxonomy.WalkReinforceMin,
		Cosine:           embed.CosineSimilarity,
	}
	h, err := harness.CreateHarness(hCfg)
	if err != nil {
		return nil, "", sirerr.Wrap(err, sirerr.CodeFailed, op, "create harness")
	}
	pipe, err := mailreport.CreatePipeline(mailreport.PipelineConfig{
		Harness: h,
		Seats:   seats,
	})
	if err != nil {
		return nil, "", err
	}
	progress := filepath.Join(repoRoot, "tmp", "mail-classify-progress.json")
	authorOnSkip := cfg.Taxonomy.AuthorOnSkip
	if os.Getenv("SHOULD_I_READ_TRY_MIN_JUDGE") == "0.80" {
		progress = filepath.Join(repoRoot, "tmp", "seek-minjudge80-progress.json")
		authorOnSkip = true
	}
	runner, err := mailreport.CreateRunner(mailreport.RunnerConfig{
		Harness:          h,
		Seats:            seats,
		Pipeline:         pipe,
		CatalogPath:      catalogPath,
		ApplyCatalog:     !cOpts.noApply,
		ProgressPath:     progress,
		ClassifyMax:      cfg.Taxonomy.ClassifyMax,
		PolypusURL:       cfg.PolypusBaseURL,
		JudgeModel:       judge,
		AuthorModel:      author,
		EmbedModel:       embedModel,
		Strategy:         cfg.Taxonomy.Strategy,
		AttachMinCosine:  cfg.Taxonomy.AttachMinCosine,
		WalkReinforceMin: cfg.Taxonomy.WalkReinforceMin,
		AuthorOnSkip:     authorOnSkip,
	}, vocab, cat)
	if err != nil {
		return nil, "", err
	}
	return runner, author, nil
}

type classifySkip struct{ err error }

func (e *classifySkip) Error() string { return e.err.Error() }
func (e *classifySkip) Unwrap() error { return e.err }

func classifySkipErr(err error) error { return &classifySkip{err: err} }

func isClassifySkip(err error) bool {
	var s *classifySkip
	return errors.As(err, &s)
}

func itemsFromFetchHunks(reader *pimdir.Reader, hunks []pimalaya.HunkRef, maxBody int) ([]mailreport.Item, int) {
	var items []mailreport.Item
	unresolved := 0
	for _, h := range hunks {
		sum, err := reader.FindByIMAPRef(h.Collection, h.ID)
		if err != nil {
			unresolved++
			continue
		}
		if strings.TrimSpace(sum.ObjectHash) == "" {
			unresolved++
			continue
		}
		items = append(items, itemFromSummary(reader, sum, maxBody))
	}
	return items, unresolved
}

func itemFromSummary(reader *pimdir.Reader, sum pimdir.EmailSummary, maxBody int) mailreport.Item {
	hash := strings.TrimSpace(sum.ObjectHash)
	item := mailreport.Item{
		ObjectHash: hash,
		Subject:    sum.Subject,
		Sender:     senderLine(sum),
		Collection: sum.Collection,
	}
	if hash == "" {
		return item
	}
	if blob, err := reader.ReadBlob(hash); err == nil {
		if prev, err := pimdir.DecodeMessagePreview(blob, maxBody); err == nil {
			item.Body = prev.Body
		}
		if hdr, err := pimdir.ParseMailHeaders(blob); err == nil {
			item.Headers = hdr
		}
	}
	return item
}

func loadReportItems(reader *pimdir.Reader, inPath string, limit int, maxBody int, collections []string) ([]mailreport.Item, error) {
	if strings.TrimSpace(inPath) != "" {
		return itemsFromExportFile(reader, inPath, maxBody)
	}
	rows, err := reader.ListRecentEmailsIn(collections, limit)
	if err != nil {
		return nil, err
	}
	var items []mailreport.Item
	for _, sum := range rows {
		items = append(items, itemFromSummary(reader, sum, maxBody))
	}
	return items, nil
}

type exportEnvelope struct {
	Emails []pimdir.EmailSummary `json:"emails"`
}

func itemsFromExportFile(reader *pimdir.Reader, path string, maxBody int) ([]mailreport.Item, error) {
	const op = "cli.itemsFromExportFile"
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "read export")
	}
	var env exportEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode export")
	}
	var items []mailreport.Item
	for _, sum := range env.Emails {
		if strings.TrimSpace(sum.ObjectHash) == "" {
			continue
		}
		items = append(items, itemFromSummary(reader, sum, maxBody))
	}
	return items, nil
}

func senderLine(s pimdir.EmailSummary) string {
	name := strings.TrimSpace(s.SenderName)
	addr := strings.TrimSpace(s.Sender)
	if name != "" && addr != "" {
		return name + " <" + addr + ">"
	}
	if name != "" {
		return name
	}
	return addr
}

func writeReportArtifact(repoRoot, outPath string, art mailreport.Artifact) (string, error) {
	const op = "cli.writeReportArtifact"
	out := strings.TrimSpace(outPath)
	if out == "" {
		tmpDir := filepath.Join(repoRoot, "tmp")
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir tmp")
		}
		out = filepath.Join(tmpDir, fmt.Sprintf("unwanted-report-%s.json", time.Now().UTC().Format("20060102T150405Z")))
	}
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "encode report")
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "write report")
	}
	return out, nil
}

func formatClassifyArtifact(art mailreport.Artifact) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Classified %d messages\n", art.Classified))
	if art.Omitted > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d omitted (cap)\n", art.Omitted)))
	}
	if art.Unresolved > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d unresolved hunks\n", art.Unresolved)))
	}
	if art.JudgeModel != "" || art.AuthorModel != "" {
		b.WriteString(fmt.Sprintf("judge %s · author %s\n", art.JudgeModel, art.AuthorModel))
	}
	b.WriteString(fmt.Sprintf("aliases added %d · new leaves %d\n", art.AliasesAdded, art.LeavesAdded))
	if art.ReportPath != "" {
		b.WriteString(art.ReportPath)
	}
	return clui.FormatBox("Classify", strings.TrimRight(b.String(), "\n"))
}

func tryMinJudgeScore() float64 {
	if os.Getenv("SHOULD_I_READ_TRY_MIN_JUDGE") != "0.80" {
		return 0
	}
	return 0.80
}
