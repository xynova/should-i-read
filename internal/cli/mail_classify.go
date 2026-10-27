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
	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"github.com/spf13/cobra"

	"github.com/behaviorengineering/taxonomy/pkg/tags"
	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	classifyProgressFileName    = "mail-classify-progress.json"
	classifyModelResolveTimeout = 45 * time.Second
	classifyJobTimeout          = 15 * time.Minute
	classifyWindowJobTimeout    = 60 * time.Minute
)

type classifyOpts struct {
	catalogPath     string
	noApply         bool
	maxBody         int
	skipAuthorProbe bool
	skipEmbedProbe  bool
}

func newMailReportCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		inPath    string
		outPath   string
		limit     int
		store     string
		catalog   string
		noApply   bool
		maxBody   int
		fromDate  string
		toDate    string
		sliceDays int
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Classify recent mail (report-only, Polypus + taxonomy)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailReport(cmd, opts, repoRoot, inPath, outPath, limit, cmd.Flags().Changed("limit"), fromDate, toDate, sliceDays, store, classifyOpts{
				catalogPath: catalog,
				noApply:     noApply,
				maxBody:     maxBody,
			})
		},
	}
	cmd.Flags().StringVar(&inPath, "in", "", "Input mail export JSON (default: list from pimdir)")
	cmd.Flags().StringVar(&outPath, "out", "", "Report path (default tmp/unwanted-report-<ts>.json)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max messages when listing from pimdir (default 25; with --from/--to, no cap unless you set --limit)")
	cmd.Flags().StringVar(&fromDate, "from", "", "Inclusive window start date (YYYY-MM-DD UTC); requires --to")
	cmd.Flags().StringVar(&toDate, "to", "", "Exclusive window end date (YYYY-MM-DD UTC); requires --from")
	cmd.Flags().IntVar(&sliceDays, "slice-days", 7, "Classify window slice length in days when --from/--to are set")
	cmd.Flags().StringVar(&store, "store", "", "Local mail store directory (overrides config)")
	cmd.Flags().StringVar(&catalog, "catalog", "", "Taxonomy catalog YAML path")
	cmd.Flags().BoolVar(&noApply, "no-apply", false, "Do not write catalog mutations to disk")
	cmd.Flags().IntVar(&maxBody, "max-body", 8192, "Max body bytes per message preview")
	return cmd
}

func runMailReport(cmd *cobra.Command, opts *rootOptions, repoRoot string, inPath, outPath string, limit int, limitChanged bool, fromDate, toDate string, sliceDays int, storeDir string, cOpts classifyOpts) error {
	const op = "cli.mail.report"
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	window, err := validateMailReportWindow(inPath, fromDate, toDate)
	if err != nil {
		return wrapCLIHop(err, op, "validate window")
	}
	if sliceDays < 1 {
		return sirerr.New(sirerr.CodeInvalid, op, "slice-days must be at least 1")
	}
	dir := storeDir
	if dir == "" {
		dir = cfg.Pimalaya.PimdirPath
	}
	reader, err := pimdir.OpenStore(dir)
	if err != nil {
		return wrapCLIHop(err, op, "open store")
	}
	runner, err := prepareClassifyRunner(cmd.Context(), cfg, repoRoot, cOpts)
	if err != nil {
		return err
	}
	defer func() { _ = runner.Close() }()
	var art mailreport.Artifact
	var sliceClusters [][]mailreport.ClusterRow
	var clusterErr error
	if !window.Zero() {
		slices, err := pimdir.SplitDateWindow(window, sliceDays)
		if err != nil {
			return wrapCLIHop(err, op, "split window")
		}
		art.WindowFrom = window.DateFrom()
		art.WindowTo = window.DateTo()
		for _, sw := range slices {
			sliceArt, err := classifyReportWindowSlice(cmd.Context(), runner, reader, inPath, sw, limit, limitChanged, cOpts.maxBody, cfg.Taxonomy.Collections)
			if err != nil {
				return wrapCLIHop(err, op, "classify slice")
			}
			mailreport.MergeClassifyArtifacts(&art, sliceArt)
		}
		tagCtx, tagCancel := context.WithTimeout(cmd.Context(), mailreport.TagPassTimeout)
		defer tagCancel()
		runner.RunTagPass(tagCtx, &art)
		for _, sw := range slices {
			reloadCtx, reloadCancel := context.WithTimeout(cmd.Context(), hostStoreOpTimeout)
			items, err := loadReportItems(reloadCtx, reader, inPath, sw, limit, limitChanged, cOpts.maxBody, cfg.Taxonomy.Collections)
			reloadCancel()
			if err != nil {
				return wrapCLIHop(err, op, "reload slice items")
			}
			hashSet := map[string]struct{}{}
			for _, it := range items {
				if h := strings.TrimSpace(it.ObjectHash); h != "" {
					hashSet[h] = struct{}{}
				}
			}
			sliceArt := mailreport.FilterArtifactByHashes(art, hashSet)
			attCtx, attCancel := context.WithTimeout(cmd.Context(), classifyJobTimeout)
			sliceArt, err = runner.ScoreAttention(attCtx, sliceArt)
			attCancel()
			if err != nil {
				return err
			}
			mailreport.MergeMessageRows(&art, sliceArt)
			clusterCtx, clusterCancel := context.WithTimeout(cmd.Context(), classifyJobTimeout)
			sliceArt, err = runner.Cluster(clusterCtx, sliceArt)
			clusterCancel()
			if err != nil {
				clusterErr = err
			}
			sliceClusters = append(sliceClusters, sliceArt.Clusters)
		}
		mergeCtx, mergeCancel := context.WithTimeout(cmd.Context(), classifyJobTimeout)
		defer mergeCancel()
		rows, clusters, err := runner.MergeClusters(mergeCtx, art.Messages, sliceClusters)
		if err != nil {
			clusterErr = err
		} else {
			art.Messages = rows
			art.Clusters = clusters
		}
	} else {
		ctx, cancel, err := classifyJobContext(cmd.Context(), false)
		if err != nil {
			return err
		}
		defer cancel()
		items, err := loadReportItems(ctx, reader, inPath, window, limit, limitChanged, cOpts.maxBody, cfg.Taxonomy.Collections)
		if err != nil {
			return wrapCLIHop(err, op, "load items")
		}
		items = mailreport.DedupeItemsByHash(items)
		art, err = runner.Classify(ctx, mailreport.SourceReport, items)
		if err != nil {
			return err
		}
		tagCtx, tagCancel := context.WithTimeout(cmd.Context(), mailreport.TagPassTimeout)
		defer tagCancel()
		runner.RunTagPass(tagCtx, &art)
		attCtx, attCancel := context.WithTimeout(cmd.Context(), classifyJobTimeout)
		defer attCancel()
		art, err = runner.ScoreAttention(attCtx, art)
		if err != nil {
			return err
		}
		art, clusterErr = runner.Cluster(attCtx, art)
	}
	path, err := writeReportArtifact(repoRoot, outPath, art)
	if err != nil {
		return wrapCLIHop(err, op, "write report")
	}
	art.ReportPath = path
	if wantJSON(cmd) {
		if err := printJSON(cmd.OutOrStdout(), art); err != nil {
			return err
		}
		if clusterErr != nil {
			return sirerr.Wrap(clusterErr, sirerr.CodeFailed, op, "cluster")
		}
		return nil
	}
	if err := writeCLILine(cmd.OutOrStdout(), formatClassifyArtifact(art)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write classify summary")
	}
	if clusterErr != nil {
		return sirerr.Wrap(clusterErr, sirerr.CodeFailed, op, "cluster")
	}
	return nil
}

func runMailClassifyAfterSync(cmd *cobra.Command, opts *rootOptions, repoRoot string, res *pimalaya.Result, noClassify bool, maxBody int, excludeCollections []string) error {
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
	items, unresolved := itemsFromFetchHunks(reader, pimalaya.FetchedHunks(res.Raw), maxBody, excludeCollections)
	ctx, cancel, err := classifyJobContext(cmd.Context(), false)
	if err != nil {
		return err
	}
	defer cancel()
	runner, err := prepareClassifyRunner(ctx, cfg, repoRoot, classifyOpts{maxBody: maxBody})
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
	defer func() { _ = runner.Close() }()
	items = mailreport.DedupeItemsByHash(items)
	art, err := runner.Classify(ctx, mailreport.SourceSyncFetch, items)
	if err != nil {
		if isClassifyUnavailable(err) {
			box := clui.FormatBox("Classify", clui.Muted("Classification skipped (Polypus unavailable)"))
			if wErr := writeCLILine(cmd.OutOrStdout(), box); wErr != nil {
				return sirerr.Wrap(wErr, sirerr.CodeFailed, op, "write classify skip")
			}
			return nil
		}
		box := clui.FormatBox("Classify", clui.Muted("Classification after sync failed: "+shortClassifyErr(err)))
		if wErr := writeCLILine(cmd.OutOrStdout(), box); wErr != nil {
			return sirerr.Wrap(wErr, sirerr.CodeFailed, op, "write classify failure")
		}
		return nil
	}
	art.Unresolved += unresolved
	art, err = runner.ScoreAttention(ctx, art)
	if err != nil {
		if isClassifyUnavailable(err) {
			box := clui.FormatBox("Classify", clui.Muted("Attention scoring failed (Polypus unavailable)"))
			if wErr := writeCLILine(cmd.OutOrStdout(), box); wErr != nil {
				return sirerr.Wrap(wErr, sirerr.CodeFailed, op, "write attention skip")
			}
		} else {
			return err
		}
	}
	art, clusterErr := runner.Cluster(ctx, art)
	path, werr := writeReportArtifact(repoRoot, "", art)
	if werr != nil {
		return sirerr.Wrap(werr, sirerr.CodeFailed, op, "write report")
	}
	art.ReportPath = path
	if wantJSON(cmd) {
		return nil
	}
	summary := formatClassifyArtifact(art)
	if clusterErr != nil {
		summary += "\n" + clui.Muted("Cluster persist failed: "+shortClassifyErr(clusterErr))
	}
	if err := writeCLILine(cmd.OutOrStdout(), summary); err != nil {
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
	var aliasJudge harness.Judge
	aliasID := strings.TrimSpace(cfg.PolypusAliasJudgeModel)
	if aliasID != "" {
		if probeErr := client.ProbeSystemOne(resolveCtx, aliasID); probeErr != nil {
			code := sirerr.CodeUnavailable
			if c, ok := sirerr.AsCode(probeErr); ok {
				code = c
			}
			return nil, sirerr.Wrap(probeErr, code, op, "configured alias judge model failed systemone probe").
				With("model", aliasID)
		}
		aliasJudge, err = mailreport.CreatePolypusJudge(client, aliasID)
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create alias judge")
		}
	}
	author, err := classifyAuthorModel(resolveCtx, client, health.ModelIDs, cfg.PolypusClassifyModel, cOpts.skipAuthorProbe)
	if err != nil {
		return nil, unavail(err, "resolve author model")
	}
	embedModel := ""
	var stropEmbedInner harness.Embedder
	essenceModel := ""
	if config.IsAttachStrategy(cfg.Taxonomy.Strategy) {
		essenceModel = author
		if em := strings.TrimSpace(cfg.PolypusEssenceModel); em != "" {
			essenceModel = em
		}
		embedFac := mailreport.NewStropEmbedFactory()
		embedModel, err = classifyEmbedModel(resolveCtx, embedFac, cfg.PolypusBaseURL, health.ModelIDs, cfg.PolypusEmbedModel, cOpts.skipEmbedProbe)
		if err != nil {
			return nil, unavail(err, "resolve embed model")
		}
		stropEmbedInner, err = newAttachEmbedInner(resolveCtx, embedFac, cfg.PolypusBaseURL, embedModel)
		if err != nil {
			return nil, err
		}
	}
	if essenceModel != "" {
		if probeErr := client.ProbeChat(resolveCtx, essenceModel); probeErr != nil {
			code := sirerr.CodeUnavailable
			if c, ok := sirerr.AsCode(probeErr); ok {
				code = c
			}
			return nil, sirerr.Wrap(probeErr, code, op, "configured essence model failed chat probe").
				With("model", essenceModel)
		}
	}
	essenceEscalate := mailreport.EffectiveEssenceEscalateModel(essenceModel, cfg.PolypusEssenceEscalateModel)
	if essenceEscalate != "" {
		if probeErr := client.ProbeChat(resolveCtx, essenceEscalate); probeErr != nil {
			code := sirerr.CodeUnavailable
			if c, ok := sirerr.AsCode(probeErr); ok {
				code = c
			}
			return nil, sirerr.Wrap(probeErr, code, op, "configured essence escalate model failed chat probe").
				With("model", essenceEscalate)
		}
	}
	var essenceBatch mailreport.ChatLineRunner
	if essenceModel != "" {
		essenceBatch, err = mailreport.NewParallelChatRunner(client, essenceModel, cfg.Taxonomy.EssenceBatchSize)
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "essence parallel runner")
		}
	}
	now := time.Now().UTC()
	essenceDumpDir := filepath.Join(repoRoot, "tmp", "essence-parse")
	var tagCatalog *tags.Catalog
	var essencePrompts mailreport.EssencePrompts
	essencePromptJoined := ""
	if essenceModel != "" {
		registerPath, err := config.DefaultTagRegisterPath()
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "tag register path")
		}
		depPath, err := config.DefaultTagDeprecationsPath()
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "tag deprecations path")
		}
		seedTags := config.SeedTagListPath(repoRoot)
		if err := tags.EnsureFiles(registerPath, depPath, seedTags); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "ensure tag files")
		}
		tagCatalog, err = tags.CreateCatalog(mailreport.MailTagCatalogConfig(registerPath, depPath, seedTags))
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "load tag catalog")
		}
		essencePrompts = mailreport.BuildEssencePrompts(tagCatalog.ActiveIDs())
		essencePromptJoined = mailreport.JoinEssencePrompts(essencePrompts)
	}
	store, err := hoststore.CreateStore(hoststore.StoreConfig{Path: cfg.StorePath, Now: now})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "open classify store")
	}
	handedOff := false
	defer func() {
		if !handedOff {
			_ = store.Close()
		}
	}()
	if err := importClassifyProgressOnce(ctx, store, repoRoot); err != nil {
		return nil, wrapCLIHop(err, op, "import progress")
	}
	var essenceCache harness.EssenceCache
	if config.IsAttachStrategy(cfg.Taxonomy.Strategy) {
		essenceCache, err = mailreport.CreateEssenceCache(store, essencePromptJoined, essenceModel, essenceEscalate)
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "essence cache")
		}
	}
	var embedSeat harness.Embedder
	if stropEmbedInner != nil {
		embedSeat, err = mailreport.CreateCachingEmbedder(mailreport.CachingEmbedderConfig{
			Inner: stropEmbedInner,
			Model: embedModel,
			Store: store,
		})
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "caching embedder")
		}
	}
	if cfg.Taxonomy.DescriptionCosineMode == config.DescriptionCosineModeRepair {
		if embedSeat == nil || strings.TrimSpace(embedModel) == "" {
			return nil, sirerr.New(sirerr.CodeInvalid, op, "description_cosine_mode repair requires polypus embed_model and embedder")
		}
	}
	seats, err := mailreport.CreateSeats(mailreport.SeatsConfig{
		Client:                     client,
		JudgeModel:                 judge,
		AuthorModel:                author,
		EssenceModel:               essenceModel,
		EssenceEscalateModel:       cfg.PolypusEssenceEscalateModel,
		EssenceDumpDir:             essenceDumpDir,
		EssencePrompts:             essencePrompts,
		TagCatalog:                 tagCatalog,
		Now:                        now,
		Embedder:                   embedSeat,
		DescriptionCosineMin:       cfg.Taxonomy.DescriptionCosineMin,
		DescriptionCosineMode:      cfg.Taxonomy.DescriptionCosineMode,
		EssenceSemanticMaxAttempts: cfg.Taxonomy.EssenceSemanticMaxAttempts,
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
		hCfg.EssenceCache = essenceCache
		hCfg.EssenceParallel = cfg.Taxonomy.EssenceBatchSize
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
	tldrModel := strings.TrimSpace(cfg.PolypusTldrModel)
	if tldrModel == "" {
		tldrModel = strings.TrimSpace(cfg.PolypusClassifyModel)
	}
	if tldrModel == "" {
		tldrModel = author
	}
	var tldrBatch mailreport.ChatLineRunner
	if essenceBatch != nil && tldrModel == essenceModel && essenceModel != "" {
		tldrBatch = essenceBatch
	} else if tldrModel != "" {
		tldrBatch, err = mailreport.NewParallelChatRunner(client, tldrModel, cfg.Taxonomy.EssenceBatchSize)
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "tldr parallel runner")
		}
	}
	attention, err := mailreport.CreateAttentionScorer(mailreport.AttentionScorerConfig{
		Client:  client,
		Model:   judge,
		TrueMin: cfg.Cluster.AttentionTrueMin,
	})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create attention scorer")
	}
	var tagCurator *mailreport.TagCurator
	if tagCatalog != nil {
		tagCurator, err = mailreport.CreateTagCurator(mailreport.TagCuratorConfig{
			Store:      store,
			Catalog:    tagCatalog,
			Client:     client,
			JudgeModel: judge,
			MinScore:   cfg.Taxonomy.MinJudgeScore,
		})
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create tag curator")
		}
	}
	runner, err := mailreport.CreateRunner(mailreport.RunnerConfig{
		Harness:                    h,
		Seats:                      seats,
		Pipeline:                   pipe,
		CatalogPath:                catalogPath,
		ApplyCatalog:               !cOpts.noApply,
		Store:                      store,
		ClassifyMax:                cfg.Taxonomy.ClassifyMax,
		PolypusURL:                 cfg.PolypusBaseURL,
		JudgeModel:                 judge,
		AuthorModel:                author,
		EmbedModel:                 embedModel,
		Strategy:                   cfg.Taxonomy.Strategy,
		AttachMinCosine:            cfg.Taxonomy.AttachMinCosine,
		WalkReinforceMin:           cfg.Taxonomy.WalkReinforceMin,
		AuthorOnSkip:               cfg.Taxonomy.AuthorOnSkip,
		EssenceBatch:               essenceBatch,
		EssenceModel:               essenceModel,
		EssenceEscalateModel:       cfg.PolypusEssenceEscalateModel,
		EssenceBatchSize:           cfg.Taxonomy.EssenceBatchSize,
		TldrBatch:                  tldrBatch,
		TldrModel:                  tldrModel,
		ClusterMinCosine:           cfg.Cluster.MinCosine,
		QuarantineTerms:            cfg.Cluster.QuarantineTerms,
		Attention:                  attention,
		AttentionTrueMin:           cfg.Cluster.AttentionTrueMin,
		EssenceDumpDir:             essenceDumpDir,
		TagCatalog:                 tagCatalog,
		TagCurator:                 tagCurator,
		AliasJudge:                 aliasJudge,
		MinJudgeScore:              cfg.Taxonomy.MinJudgeScore,
		DescriptionCosineMin:       cfg.Taxonomy.DescriptionCosineMin,
		DescriptionCosineMode:      cfg.Taxonomy.DescriptionCosineMode,
		EssenceSemanticMaxAttempts: cfg.Taxonomy.EssenceSemanticMaxAttempts,
		Now:                        now,
	}, vocab, cat)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create runner")
	}
	handedOff = true
	return runner, nil
}

func importClassifyProgressOnce(ctx context.Context, store *hoststore.Store, repoRoot string) error {
	const op = "cli.importClassifyProgressOnce"
	progress := filepath.Join(repoRoot, "tmp", classifyProgressFileName)
	pf, err := mailreport.LoadProgress(progress)
	if err != nil {
		return err
	}
	if len(pf.Entries) == 0 {
		return nil
	}
	_, err = store.ImportProgress(ctx, mailreport.ProgressEntriesForImport(pf.Entries))
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "import progress")
	}
	return nil
}

func isClassifyUnavailable(err error) bool {
	code, ok := sirerr.AsCode(err)
	return ok && code == sirerr.CodeUnavailable
}

func shortClassifyErr(err error) string {
	if err == nil {
		return ""
	}
	s := strings.TrimSpace(err.Error())
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

func itemsFromFetchHunks(reader *pimdir.Reader, hunks []pimalaya.HunkRef, maxBody int, excludeCollections []string) ([]mailreport.Item, int) {
	var items []mailreport.Item
	unresolved := 0
	for _, h := range hunks {
		if pimalaya.CollectionExcluded(h.Collection, excludeCollections) {
			continue
		}
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

func validateMailReportWindow(inPath, fromDate, toDate string) (pimdir.DateWindow, error) {
	const op = "cli.validateMailReportWindow"
	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)
	if strings.TrimSpace(inPath) != "" && (fromDate != "" || toDate != "") {
		return pimdir.DateWindow{}, sirerr.New(sirerr.CodeInvalid, op, "--in cannot be used with --from or --to")
	}
	if fromDate == "" && toDate == "" {
		return pimdir.DateWindow{}, nil
	}
	window, err := pimdir.CreateDateWindow(fromDate, toDate)
	if err != nil {
		return pimdir.DateWindow{}, wrapCLIHop(err, op, "parse --from/--to window")
	}
	return window, nil
}

func reportListLimit(limit int, limitChanged bool, window pimdir.DateWindow) int {
	if window.Zero() {
		return limit
	}
	if limitChanged {
		return limit
	}
	return 0
}

func loadReportItems(ctx context.Context, reader *pimdir.Reader, inPath string, window pimdir.DateWindow, limit int, limitChanged bool, maxBody int, collections []string) ([]mailreport.Item, error) {
	if strings.TrimSpace(inPath) != "" {
		return itemsFromExportFile(reader, inPath, maxBody)
	}
	listLimit := reportListLimit(limit, limitChanged, window)
	var rows []pimdir.EmailSummary
	var err error
	if !window.Zero() {
		rows, err = reader.ListEmailsIn(ctx, collections, window, listLimit)
	} else {
		rows, err = reader.ListRecentEmailsIn(ctx, collections, listLimit)
	}
	if err != nil {
		return nil, wrapCLIHop(err, "cli.loadReportItems", "list mail")
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
		stamp, err := reportFileStamp(art.GeneratedAt)
		if err != nil {
			return "", wrapCLIHop(err, op, "report stamp")
		}
		tmpDir := filepath.Join(repoRoot, "tmp")
		if err := os.MkdirAll(tmpDir, 0o750); err != nil {
			return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir tmp")
		}
		out = filepath.Join(tmpDir, fmt.Sprintf("unwanted-report-%s.json", stamp))
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

const (
	humanLabelYes = "yes"
	humanLabelNo  = "no"
)

func humanBoolLabel(v bool) string {
	if v {
		return humanLabelYes
	}
	return humanLabelNo
}

func hasEssenceParseError(art mailreport.Artifact) bool {
	return essenceParseErrorCount(art) > 0
}

func essenceParseErrorCount(art mailreport.Artifact) int {
	n := 0
	for _, m := range art.Messages {
		if strings.Contains(m.Error, "essence parse") {
			n++
		}
	}
	return n
}

func classifyFailedMessages(art mailreport.Artifact) []mailreport.MessageRow {
	var failed []mailreport.MessageRow
	for _, m := range art.Messages {
		if strings.TrimSpace(m.Error) != "" {
			failed = append(failed, m)
		}
	}
	return failed
}

func classifyEmbedModel(ctx context.Context, fac mailreport.StropEmbedFactory, baseURL string, catalogIDs []string, embedYAML string, skipProbe bool) (string, error) {
	if skipProbe {
		return mailreport.PickEmbedModel(catalogIDs, embedYAML)
	}
	return mailreport.ResolveEmbedModel(ctx, fac, baseURL, catalogIDs, embedYAML)
}

func classifyAuthorModel(ctx context.Context, prober mailreport.ChatProber, catalogIDs []string, authorYAML string, skipProbe bool) (string, error) {
	if skipProbe {
		return mailreport.PickAuthorModel(catalogIDs, authorYAML)
	}
	model, _, err := mailreport.ResolveAuthorModel(ctx, prober, catalogIDs, authorYAML)
	return model, err
}

func glossClassifyMsgErr(errMsg string) string {
	return mailreport.ClassifyFailureCause(errMsg)
}

func truncateClassifySubject(subject string, max int) string {
	return truncateRunes(subject, max)
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func formatClassifyTLDR(tldr string) string {
	const indent = "    "
	if strings.TrimSpace(tldr) == "" {
		return ""
	}
	wrapped := clui.WrapToWidth(tldr, clui.BoxContentWidth()-len(indent))
	var b strings.Builder
	for i, line := range strings.Split(wrapped, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(indent)
		b.WriteString(line)
	}
	return b.String()
}

func jobContextIncomplete(ctx context.Context) string {
	if ctx == nil || ctx.Err() == nil {
		return ""
	}
	return ctx.Err().Error()
}

func classifyReportWindowSlice(
	cmdCtx context.Context,
	runner *mailreport.Runner,
	reader *pimdir.Reader,
	inPath string,
	sw pimdir.DateWindow,
	limit int,
	limitChanged bool,
	maxBody int,
	collections []string,
) (mailreport.Artifact, error) {
	sliceCtx, cancel, err := classifyJobContext(cmdCtx, true)
	if err != nil {
		return mailreport.Artifact{}, err
	}
	defer cancel()
	items, err := loadReportItems(sliceCtx, reader, inPath, sw, limit, limitChanged, maxBody, collections)
	if err != nil {
		return mailreport.Artifact{}, err
	}
	items = mailreport.DedupeItemsByHash(items)
	sliceArt, err := runner.Classify(sliceCtx, mailreport.SourceReport, items)
	if err != nil {
		return mailreport.Artifact{}, err
	}
	sliceArt.IncompleteError = jobContextIncomplete(sliceCtx)
	return sliceArt, nil
}

func classifyJobContext(parent context.Context, windowed bool) (context.Context, context.CancelFunc, error) {
	const op = "cli.classifyJobContext"
	if parent == nil {
		return nil, func() {}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	timeout := classifyJobTimeout
	if windowed {
		timeout = classifyWindowJobTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return ctx, cancel, nil
}

func wrapCLIHop(err error, op, msg string) error {
	if err == nil {
		return nil
	}
	code := sirerr.CodeFailed
	if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeInvalid {
		code = sirerr.CodeInvalid
	}
	return sirerr.Wrap(err, code, op, msg)
}

func reportFileStamp(generatedAt string) (string, error) {
	const op = "cli.reportFileStamp"
	raw := strings.TrimSpace(generatedAt)
	if raw == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "generated_at is empty")
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeInvalid, op, "parse generated_at")
	}
	return t.UTC().Format("20060102T150405Z"), nil
}

func formatClassifyArtifact(art mailreport.Artifact) string {
	var b strings.Builder
	if art.WindowFrom != "" && art.WindowTo != "" {
		fmt.Fprintf(&b, "Window [%s, %s) UTC\n", art.WindowFrom, art.WindowTo)
	}
	failed := classifyFailedMessages(art)
	fmt.Fprintf(&b, "Classified %d", art.Classified)
	if len(failed) > 0 {
		fmt.Fprintf(&b, " · failed %d", len(failed))
	}
	b.WriteByte('\n')
	if art.Omitted > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d omitted (cap)\n", art.Omitted)))
	}
	if art.AttentionOmitted > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d attention omitted (cap)\n", art.AttentionOmitted)))
	}
	if art.Unresolved > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d unresolved hunks\n", art.Unresolved)))
	}
	if art.JudgeModel != "" || art.AuthorModel != "" {
		fmt.Fprintf(&b, "judge %s · author %s\n", art.JudgeModel, art.AuthorModel)
	}
	fmt.Fprintf(&b, "aliases added %d · new leaves %d\n", art.AliasesAdded, art.LeavesAdded)
	embedMiss := 0
	for _, m := range art.Messages {
		if strings.TrimSpace(m.EmbedPersist) != "" {
			embedMiss++
		}
	}
	if embedMiss > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d classified without stored embedding\n", embedMiss)))
	}
	if art.EssenceDumpDir != "" && hasEssenceParseError(art) {
		n := essenceParseErrorCount(art)
		b.WriteString(clui.Muted(fmt.Sprintf("essence dumps %d · %s\n", n, art.EssenceDumpDir)))
	}
	if len(failed) > 0 {
		fmt.Fprintf(&b, "\nFailed %d\n", len(failed))
		for _, m := range failed {
			label := truncateClassifySubject(m.Subject, 56)
			if label == "" {
				label = m.ObjectHash
			}
			fmt.Fprintf(&b, "  %s  %s\n", label, glossClassifyMsgErr(m.Error))
		}
	}
	if len(art.Clusters) > 0 {
		fmt.Fprintf(&b, "%d clusters\n", len(art.Clusters))
		for _, c := range art.Clusters {
			label := c.TermID
			if label == "" && c.SenderDomain != "" {
				label = c.SenderDomain
			}
			if label == "" {
				label = c.ID
			}
			tldr := strings.TrimSpace(c.TLDR)
			if tldr == "" && c.TLDRError != "" {
				tldr = clui.Muted("(no tldr)")
			}
			if c.Attention != nil {
				reply := humanBoolLabel(c.Attention.NeedsReply)
				decision := humanBoolLabel(c.Attention.InvolvesDecision)
				fmt.Fprintf(&b, "  %s (%d) %s  urg=%.2f reply=%s decision=%s\n",
					label, c.Size, c.SuggestedAction, c.Attention.Urgency, reply, decision)
			} else {
				fmt.Fprintf(&b, "  %s (%d) %s\n", label, c.Size, c.SuggestedAction)
			}
			if tldrLine := formatClassifyTLDR(tldr); tldrLine != "" {
				b.WriteString(tldrLine)
				b.WriteByte('\n')
			}
		}
	}
	if art.ReportPath != "" {
		b.WriteString(art.ReportPath)
	}
	return clui.FormatBox("Classify", strings.TrimRight(b.String(), "\n"))
}
