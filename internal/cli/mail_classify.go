package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/strop/pkg/embed"
	"github.com/behaviorengineering/strop/pkg/openaibatch"
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

const (
	classifyProgressFileName    = "mail-classify-progress.json"
	classifyModelResolveTimeout = 45 * time.Second
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
	runner, err := prepareClassifyRunner(cmd.Context(), cfg, repoRoot, cOpts)
	if err != nil {
		return err
	}
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
	if err := writeCLILine(cmd.OutOrStdout(), formatClassifyArtifact(art)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write classify summary")
	}
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
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "open store")
	}
	items, unresolved := itemsFromFetchHunks(reader, pimalaya.FetchedHunks(res.Raw), maxBody)
	runner, err := prepareClassifyRunner(cmd.Context(), cfg, repoRoot, classifyOpts{maxBody: maxBody})
	if err != nil {
		if isClassifyUnavailable(err) {
			box := clui.FormatBox("Classify", clui.Muted("Classification skipped (Polypus unavailable)"))
			if wErr := writeCLILine(cmd.OutOrStdout(), box); wErr != nil {
				return sirerr.Wrap(wErr, sirerr.CodeFailed, op, "write classify skip")
			}
			return nil
		}
		return err
	}
	items = mailreport.DedupeItemsByHash(items)
	art, err := runner.Classify(cmd.Context(), mailreport.SourceSyncFetch, items)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "classify after sync")
	}
	art.Unresolved += unresolved
	path, werr := writeReportArtifact(repoRoot, "", art)
	if werr != nil {
		return sirerr.Wrap(werr, sirerr.CodeFailed, op, "write report")
	}
	art.ReportPath = path
	if wantJSON(cmd) {
		return nil
	}
	if err := writeCLILine(cmd.OutOrStdout(), formatClassifyArtifact(art)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write classify summary")
	}
	return nil
}

func prepareClassifyRunner(ctx context.Context, cfg config.Config, repoRoot string, cOpts classifyOpts) (*mailreport.Runner, error) {
	const op = "cli.prepareClassifyRunner"
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	unavail := func(cause error, msg string) error {
		return sirerr.Wrap(cause, sirerr.CodeUnavailable, op, msg)
	}
	catalogPath := strings.TrimSpace(cOpts.catalogPath)
	if catalogPath == "" {
		catalogPath = cfg.Taxonomy.CatalogPath
	}
	seed := config.SeedCatalogPath(repoRoot)
	if config.IsAttachStrategy(cfg.Taxonomy.Strategy) {
		seed = config.SeedKindCatalogPath(repoRoot)
	}
	if err := mailreport.EnsureCatalog(catalogPath, seed); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "ensure catalog")
	}
	cat, vocab, err := mailreport.LoadCatalog(catalogPath)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "load catalog")
	}
	client, err := polypus.Create(cfg.PolypusBaseURL, nil)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create polypus client")
	}
	health, err := client.Check(ctx)
	if err != nil {
		return nil, unavail(err, "polypus check")
	}
	resolveCtx, cancel := context.WithTimeout(ctx, classifyModelResolveTimeout)
	defer cancel()
	judge, _, err := mailreport.ResolveJudgeModel(resolveCtx, client, health.ModelIDs, cfg.PolypusJudgeModel)
	if err != nil {
		return nil, unavail(err, "resolve judge model")
	}
	author, _, err := mailreport.ResolveAuthorModel(resolveCtx, client, health.ModelIDs, cfg.PolypusClassifyModel)
	if err != nil {
		return nil, unavail(err, "resolve author model")
	}
	embedModel := ""
	var embedSeat harness.Embedder
	essenceModel := ""
	if config.IsAttachStrategy(cfg.Taxonomy.Strategy) {
		essenceModel = author
		embedFac := mailreport.NewStropEmbedFactory()
		embedModel, err = mailreport.ResolveEmbedModel(resolveCtx, embedFac, cfg.PolypusBaseURL, health.ModelIDs, cfg.PolypusEmbedModel)
		if err != nil {
			return nil, unavail(err, "resolve embed model")
		}
		stropEmb, err := embedFac.CreateEmbedder(resolveCtx, cfg.PolypusBaseURL, embedModel)
		if err != nil {
			return nil, unavail(err, "create embedder")
		}
		embedSeat = mailreport.NewStropEmbedSeat(stropEmb, embedModel)
	}
	var essenceBatch mailreport.ChatLineRunner
	if essenceModel != "" {
		oa := openaibatch.Client{BaseURL: cfg.PolypusBaseURL, HTTPClient: client.HTTPClient}
		essenceBatch, err = mailreport.NewPreferBatchChatRunner(mailreport.NoopSubLLM(), oa, essenceModel)
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "essence batch runner")
		}
	}
	seats, err := mailreport.CreateSeats(mailreport.SeatsConfig{
		Client:       client,
		JudgeModel:   judge,
		AuthorModel:  author,
		EssenceModel: essenceModel,
		Embedder:     embedSeat,
	})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create seats")
	}
	authorSeat := seats.Author
	if !cfg.Taxonomy.AuthorOnSkip {
		authorSeat = mailreport.NoopAuthor()
	}
	hCfg := harness.Config{
		Judge:         seats.Judge,
		Author:        authorSeat,
		MinJudgeScore: cfg.Taxonomy.MinJudgeScore,
	}
	if config.IsAttachStrategy(cfg.Taxonomy.Strategy) {
		hCfg.Strategy = harness.StrategyAttach
		hCfg.Embedder = seats.Embedder
		hCfg.Essencer = seats.Essencer
		hCfg.AttachMinCosine = cfg.Taxonomy.AttachMinCosine
		hCfg.WalkReinforceMin = cfg.Taxonomy.WalkReinforceMin
		hCfg.Cosine = embed.CosineSimilarity
	}
	h, err := harness.CreateHarness(hCfg)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create harness")
	}
	pipe, err := mailreport.CreatePipeline(mailreport.PipelineConfig{
		Harness: h,
		Seats:   seats,
	})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create pipeline")
	}
	progress := filepath.Join(repoRoot, "tmp", classifyProgressFileName)
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
		AuthorOnSkip:     cfg.Taxonomy.AuthorOnSkip,
		EssenceBatch:     essenceBatch,
		EssenceModel:     essenceModel,
		EssenceBatchSize: cfg.Taxonomy.EssenceBatchSize,
	}, vocab, cat)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create runner")
	}
	return runner, nil
}

func isClassifyUnavailable(err error) bool {
	code, ok := sirerr.AsCode(err)
	return ok && code == sirerr.CodeUnavailable
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
	//nolint:gosec // operator-supplied mail export path (--in)
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
		if err := os.MkdirAll(tmpDir, 0o750); err != nil {
			return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir tmp")
		}
		out = filepath.Join(tmpDir, fmt.Sprintf("unwanted-report-%s.json", time.Now().UTC().Format("20060102T150405Z")))
	}
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "encode report")
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0o600); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "write report")
	}
	return out, nil
}

func formatClassifyArtifact(art mailreport.Artifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Classified %d messages\n", art.Classified)
	if art.Omitted > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d omitted (cap)\n", art.Omitted)))
	}
	if art.Unresolved > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d unresolved hunks\n", art.Unresolved)))
	}
	if art.JudgeModel != "" || art.AuthorModel != "" {
		fmt.Fprintf(&b, "judge %s · author %s\n", art.JudgeModel, art.AuthorModel)
	}
	fmt.Fprintf(&b, "aliases added %d · new leaves %d\n", art.AliasesAdded, art.LeavesAdded)
	if art.ReportPath != "" {
		b.WriteString(art.ReportPath)
	}
	return clui.FormatBox("Classify", strings.TrimRight(b.String(), "\n"))
}
