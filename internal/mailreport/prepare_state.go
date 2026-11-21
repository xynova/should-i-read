package mailreport

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/strop/pkg/embed"
	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"github.com/behaviorengineering/taxonomy/pkg/tags"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type prepareState struct {
	op                  string
	now                 time.Time
	cfg                 config.Config
	opts                PrepareOpts
	catalogPath         string
	cat                 *catalog.Catalog
	vocab               catalog.Vocabulary
	client              *polypus.Client
	health              *polypus.HealthResult
	judge               string
	author              string
	aliasJudge          harness.Judge
	embedModel          string
	essenceModel        string
	essenceBatch        ChatLineRunner
	stropEmbedInner     harness.Embedder
	essenceCache        harness.EssenceCache
	embedSeat           harness.Embedder
	essenceDumpDir      string
	tagCatalog          *tags.Catalog
	essencePrompts      EssencePrompts
	essencePromptJoined string
	store               *hoststore.Store
	seats               Seats
	h                   *harness.Harness
	tldrBatch           ChatLineRunner
	tldrModel           string
	attention           AttentionClient
	tagCurator          *TagCurator
}

func (p *prepareState) loadCatalog() error {
	p.catalogPath = strings.TrimSpace(p.opts.CatalogPath)
	if p.catalogPath == "" {
		p.catalogPath = p.cfg.Taxonomy.CatalogPath
	}
	seed := config.SeedCatalogPath(p.opts.RepoRoot)
	if config.IsAttachStrategy(p.cfg.Taxonomy.Strategy) {
		seed = config.SeedKindCatalogPath(p.opts.RepoRoot)
	}
	if err := EnsureCatalog(p.catalogPath, seed); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "ensure catalog")
	}
	cat, vocab, err := LoadCatalog(p.catalogPath)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "load catalog")
	}
	p.cat = cat
	p.vocab = vocab
	return nil
}

func (p *prepareState) connectPolypus(ctx context.Context) error {
	client, err := polypus.Create(p.cfg.PolypusBaseURL, nil)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "create polypus client")
	}
	health, err := client.Check(ctx)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, p.op, "polypus check")
	}
	p.client = client
	p.health = health
	return nil
}

func (p *prepareState) resolveModels(ctx context.Context) error {
	judge, _, err := ResolveJudgeModel(ctx, p.client, p.health.ModelIDs, p.cfg.PolypusJudgeModel)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, p.op, "resolve judge model")
	}
	p.judge = judge
	if err := p.resolveAliasJudge(ctx); err != nil {
		return err
	}
	author, err := pickOrResolveAuthor(ctx, p.client, p.health.ModelIDs, p.cfg.PolypusClassifyModel, p.opts.SkipAuthorProbe)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, p.op, "resolve author model")
	}
	p.author = author
	return p.resolveAttachModels(ctx)
}

func (p *prepareState) resolveAliasJudge(ctx context.Context) error {
	aliasID := strings.TrimSpace(p.cfg.PolypusAliasJudgeModel)
	if aliasID == "" {
		return nil
	}
	if probeErr := p.client.ProbeSystemOne(ctx, aliasID); probeErr != nil {
		code := sirerr.CodeUnavailable
		if c, ok := sirerr.AsCode(probeErr); ok {
			code = c
		}
		return sirerr.Wrap(probeErr, code, p.op, "configured alias judge model failed systemone probe").
			With("model", aliasID)
	}
	aliasJudge, err := CreatePolypusJudge(p.client, aliasID)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "create alias judge")
	}
	p.aliasJudge = aliasJudge
	return nil
}

func (p *prepareState) resolveAttachModels(ctx context.Context) error {
	if !config.IsAttachStrategy(p.cfg.Taxonomy.Strategy) {
		return nil
	}
	p.essenceModel = p.author
	if em := strings.TrimSpace(p.cfg.PolypusEssenceModel); em != "" {
		p.essenceModel = em
	}
	embedFac := NewStropEmbedFactory()
	embedModel, err := pickOrResolveEmbed(ctx, embedFac, p.cfg.PolypusBaseURL, p.health.ModelIDs, p.cfg.PolypusEmbedModel, p.opts.SkipEmbedProbe)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, p.op, "resolve embed model")
	}
	p.embedModel = embedModel
	inner, err := newAttachEmbedInner(ctx, embedFac, p.cfg.PolypusBaseURL, embedModel)
	if err != nil {
		return err
	}
	p.stropEmbedInner = inner
	if err := probeChatModel(ctx, p.client, p.essenceModel, p.op, "configured essence model failed chat probe"); err != nil {
		return err
	}
	essenceEscalate := EffectiveEssenceEscalateModel(p.essenceModel, p.cfg.PolypusEssenceEscalateModel)
	if err := probeChatModel(ctx, p.client, essenceEscalate, p.op, "configured essence escalate model failed chat probe"); err != nil {
		return err
	}
	if p.essenceModel == "" {
		return nil
	}
	batch, err := NewParallelChatRunner(p.client, p.essenceModel, p.cfg.Taxonomy.EssenceBatchSize)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "essence parallel runner")
	}
	p.essenceBatch = batch
	return nil
}

func probeChatModel(ctx context.Context, client *polypus.Client, model, op, msg string) error {
	if strings.TrimSpace(model) == "" {
		return nil
	}
	if probeErr := client.ProbeChat(ctx, model); probeErr != nil {
		code := sirerr.CodeUnavailable
		if c, ok := sirerr.AsCode(probeErr); ok {
			code = c
		}
		return sirerr.Wrap(probeErr, code, op, msg).With("model", model)
	}
	return nil
}

func (p *prepareState) loadTags() error {
	p.essenceDumpDir = filepath.Join(p.opts.RepoRoot, "tmp", "essence-parse")
	if p.essenceModel == "" {
		return nil
	}
	registerPath, err := config.DefaultTagRegisterPath()
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "tag register path")
	}
	depPath, err := config.DefaultTagDeprecationsPath()
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "tag deprecations path")
	}
	seedTags := config.SeedTagListPath(p.opts.RepoRoot)
	if err := tags.EnsureFiles(registerPath, depPath, seedTags); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "ensure tag files")
	}
	tagCatalog, err := tags.CreateCatalog(MailTagCatalogConfig(registerPath, depPath, seedTags))
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "load tag catalog")
	}
	p.tagCatalog = tagCatalog
	p.essencePrompts = BuildEssencePrompts(tagCatalog.ActiveIDs())
	p.essencePromptJoined = JoinEssencePrompts(p.essencePrompts)
	return nil
}

func (p *prepareState) openStore(ctx context.Context) error {
	store, err := hoststore.CreateStore(hoststore.StoreConfig{Path: p.cfg.StorePath, Now: p.now})
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "open classify store")
	}
	p.store = store
	if err := importClassifyProgressOnce(ctx, store, p.opts.RepoRoot); err != nil {
		return err
	}
	return p.wireCaches()
}

func (p *prepareState) wireCaches() error {
	if config.IsAttachStrategy(p.cfg.Taxonomy.Strategy) {
		essenceCache, err := CreateEssenceCache(p.store, p.essencePromptJoined, p.essenceModel, EffectiveEssenceEscalateModel(p.essenceModel, p.cfg.PolypusEssenceEscalateModel))
		if err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "essence cache")
		}
		p.essenceCache = essenceCache
	}
	if p.stropEmbedInner == nil {
		return nil
	}
	embedSeat, err := CreateCachingEmbedder(CachingEmbedderConfig{
		Inner: p.stropEmbedInner,
		Model: p.embedModel,
		Store: p.store,
	})
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "caching embedder")
	}
	p.embedSeat = embedSeat
	return nil
}

func (p *prepareState) buildHarness() error {
	if p.cfg.Taxonomy.DescriptionCosineMode == config.DescriptionCosineModeRepair {
		if p.embedSeat == nil || strings.TrimSpace(p.embedModel) == "" {
			return sirerr.New(sirerr.CodeInvalid, p.op, "description_cosine_mode repair requires polypus embed_model and embedder")
		}
	}
	seats, err := CreateSeats(SeatsConfig{
		Client:                     p.client,
		JudgeModel:                 p.judge,
		AuthorModel:                p.author,
		EssenceModel:               p.essenceModel,
		EssenceEscalateModel:       p.cfg.PolypusEssenceEscalateModel,
		EssenceDumpDir:             p.essenceDumpDir,
		EssencePrompts:             p.essencePrompts,
		TagCatalog:                 p.tagCatalog,
		Now:                        p.now,
		Embedder:                   p.embedSeat,
		DescriptionCosineMin:       p.cfg.Taxonomy.DescriptionCosineMin,
		DescriptionCosineMode:      p.cfg.Taxonomy.DescriptionCosineMode,
		EssenceSemanticMaxAttempts: p.cfg.Taxonomy.EssenceSemanticMaxAttempts,
		Events:                     classifyEvents(p.opts.Events, p.now),
	})
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "create seats")
	}
	p.seats = seats
	authorSeat := seats.Author
	if !p.cfg.Taxonomy.AuthorOnSkip {
		authorSeat = NoopAuthor()
	}
	hCfg := harness.Config{
		Judge:         seats.Judge,
		Author:        authorSeat,
		MinJudgeScore: p.cfg.Taxonomy.MinJudgeScore,
	}
	if config.IsAttachStrategy(p.cfg.Taxonomy.Strategy) {
		hCfg.Strategy = harness.StrategyAttach
		hCfg.Embedder = seats.Embedder
		hCfg.Essencer = seats.Essencer
		hCfg.AttachMinCosine = p.cfg.Taxonomy.AttachMinCosine
		hCfg.WalkReinforceMin = p.cfg.Taxonomy.WalkReinforceMin
		hCfg.Cosine = embed.CosineSimilarity
		hCfg.EssenceCache = p.essenceCache
		hCfg.EssenceParallel = p.cfg.Taxonomy.EssenceBatchSize
	}
	h, err := harness.CreateHarness(hCfg)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "create harness")
	}
	p.h = h
	return nil
}

func (p *prepareState) buildPostSeats() error {
	tldrModel := strings.TrimSpace(p.cfg.PolypusTldrModel)
	if tldrModel == "" {
		tldrModel = strings.TrimSpace(p.cfg.PolypusClassifyModel)
	}
	if tldrModel == "" {
		tldrModel = p.author
	}
	p.tldrModel = tldrModel
	if p.essenceBatch != nil && tldrModel == p.essenceModel && p.essenceModel != "" {
		p.tldrBatch = p.essenceBatch
	} else if tldrModel != "" {
		batch, err := NewParallelChatRunner(p.client, tldrModel, p.cfg.Taxonomy.EssenceBatchSize)
		if err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "tldr parallel runner")
		}
		p.tldrBatch = batch
	}
	attention, err := CreateAttentionScorer(AttentionScorerConfig{
		Client:  p.client,
		Model:   p.judge,
		TrueMin: p.cfg.Cluster.AttentionTrueMin,
	})
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "create attention scorer")
	}
	p.attention = attention
	if p.tagCatalog == nil {
		return nil
	}
	tagCurator, err := CreateTagCurator(TagCuratorConfig{
		Store:      p.store,
		Catalog:    p.tagCatalog,
		Client:     p.client,
		JudgeModel: p.judge,
		MinScore:   p.cfg.Taxonomy.MinJudgeScore,
	})
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, p.op, "create tag curator")
	}
	p.tagCurator = tagCurator
	return nil
}

func (p *prepareState) createRunner() (*Runner, error) {
	runner, err := CreateRunner(RunnerConfig{
		Harness:                    p.h,
		Seats:                      p.seats,
		CatalogPath:                p.catalogPath,
		ApplyCatalog:               !p.opts.NoApply,
		Store:                      p.store,
		ClassifyMax:                p.cfg.Taxonomy.ClassifyMax,
		PolypusURL:                 p.cfg.PolypusBaseURL,
		JudgeModel:                 p.judge,
		AuthorModel:                p.author,
		EmbedModel:                 p.embedModel,
		Strategy:                   p.cfg.Taxonomy.Strategy,
		AttachMinCosine:            p.cfg.Taxonomy.AttachMinCosine,
		WalkReinforceMin:           p.cfg.Taxonomy.WalkReinforceMin,
		AuthorOnSkip:               p.cfg.Taxonomy.AuthorOnSkip,
		EssenceBatch:               p.essenceBatch,
		EssenceModel:               p.essenceModel,
		EssenceEscalateModel:       p.cfg.PolypusEssenceEscalateModel,
		EssenceBatchSize:           p.cfg.Taxonomy.EssenceBatchSize,
		TldrBatch:                  p.tldrBatch,
		TldrModel:                  p.tldrModel,
		ClusterMinCosine:           p.cfg.Cluster.MinCosine,
		QuarantineTerms:            p.cfg.Cluster.QuarantineTerms,
		Attention:                  p.attention,
		AttentionTrueMin:           p.cfg.Cluster.AttentionTrueMin,
		EssenceDumpDir:             p.essenceDumpDir,
		TagCatalog:                 p.tagCatalog,
		TagCurator:                 p.tagCurator,
		AliasJudge:                 p.aliasJudge,
		MinJudgeScore:              p.cfg.Taxonomy.MinJudgeScore,
		DescriptionCosineMin:       p.cfg.Taxonomy.DescriptionCosineMin,
		DescriptionCosineMode:      p.cfg.Taxonomy.DescriptionCosineMode,
		EssenceSemanticMaxAttempts: p.cfg.Taxonomy.EssenceSemanticMaxAttempts,
		ShapeConfirm:               p.cfg.Taxonomy.ShapeConfirm,
		ShapeMinRepeats:            p.cfg.Taxonomy.ShapeMinRepeats,
		ShapeConfirmMin:            p.cfg.Taxonomy.ShapeConfirmMin,
		ShapeAllowedLeaves:         p.cfg.Taxonomy.ShapeAllowedLeaves,
		EssenceSystemPrompt:        p.essencePromptJoined,
		Now:                        p.now,
		WorthReadPrior:             p.cfg.Taxonomy.WorthReadPrior,
		WorthReadMinNeighbors:      p.cfg.Taxonomy.WorthReadMinNeighbors,
		WorthReadMinCosine:         p.cfg.Taxonomy.WorthReadMinCosine,
		WorthReadSimilarK:          p.cfg.Taxonomy.WorthReadSimilarK,
	}, p.vocab, p.cat)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, p.op, "create runner")
	}
	return runner, nil
}
