package mailreport

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/behaviorengineering/taxonomy/pkg/tags"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	operateTimeoutWalk   = 60 * time.Second
	operateTimeoutAttach = 180 * time.Second
)

// RunnerConfig configures classify runs.
type RunnerConfig struct {
	Harness              *harness.Harness
	Seats                Seats
	Pipeline             *Pipeline
	CatalogPath          string
	ApplyCatalog         bool
	Store                *hoststore.Store
	ClassifyMax          int
	PolypusURL           string
	JudgeModel           string
	AuthorModel          string
	AuthorOnSkip         bool
	Strategy             string
	EmbedModel           string
	AttachMinCosine      float64
	WalkReinforceMin     float64
	EssenceBatch         ChatLineRunner
	EssenceModel         string
	EssenceEscalateModel string
	EssenceBatchSize     int
	TldrBatch            ChatLineRunner
	TldrModel            string
	ClusterMinCosine     float64
	QuarantineTerms      []string
	Attention            AttentionClient
	AttentionTrueMin     float64
	EssenceDumpDir       string
	EssenceSystemPrompt  string
	TagCatalog           *tags.Catalog
	TagCurator           *TagCurator
	Now                  time.Time
}

// Runner classifies items with taxonomy Operate.
type Runner struct {
	h                    *harness.Harness
	seats                Seats
	pipe                 *Pipeline
	catalogPath          string
	apply                bool
	store                *hoststore.Store
	max                  int
	polypusURL           string
	judgeModel           string
	authorModel          string
	authorOnSkip         bool
	vocab                catalog.Vocabulary
	cat                  *catalog.Catalog
	strategy             string
	embedModel           string
	attachMinCosine      float64
	walkReinforceMin     float64
	operateWorld         string
	essenceBatch         ChatLineRunner
	essenceModel         string
	essenceEscalateModel string
	essenceBatchSize     int
	prefetchByHash       map[string]*harness.EssenceOut
	tldrBatch            ChatLineRunner
	tldrModel            string
	clusterMinCosine     float64
	quarantineTerms      []string
	attention            AttentionClient
	attentionTrueMin     float64
	essenceDumpDir       string
	essenceSystemPrompt  string
	tagCatalog           *tags.Catalog
	tagCurator           *TagCurator
	aiHops               int
	now                  time.Time
}

// CreateRunner builds a Runner after catalog is loaded.
func CreateRunner(cfg RunnerConfig, vocab catalog.Vocabulary, cat *catalog.Catalog) (*Runner, error) {
	const op = "mailreport.CreateRunner"
	if cfg.Harness == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil harness")
	}
	if cat == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil catalog")
	}
	if cfg.Store == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if cfg.Pipeline != nil && cfg.Pipeline.Jobs == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "pipeline missing job runner")
	}
	max := cfg.ClassifyMax
	if max <= 0 {
		max = 50
	}
	strategy, err := config.NormalizeTaxonomyStrategy(cfg.Strategy)
	if err != nil {
		return nil, err
	}
	world := WorldContext
	if config.IsAttachStrategy(strategy) {
		world = WorldContextAttach
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &Runner{
		h:                    cfg.Harness,
		seats:                cfg.Seats,
		pipe:                 cfg.Pipeline,
		catalogPath:          cfg.CatalogPath,
		apply:                cfg.ApplyCatalog,
		store:                cfg.Store,
		max:                  max,
		polypusURL:           cfg.PolypusURL,
		judgeModel:           strings.TrimSpace(cfg.JudgeModel),
		authorModel:          strings.TrimSpace(cfg.AuthorModel),
		embedModel:           strings.TrimSpace(cfg.EmbedModel),
		authorOnSkip:         cfg.AuthorOnSkip,
		vocab:                vocab,
		cat:                  cat,
		strategy:             strategy,
		attachMinCosine:      cfg.AttachMinCosine,
		walkReinforceMin:     cfg.WalkReinforceMin,
		operateWorld:         world,
		essenceBatch:         cfg.EssenceBatch,
		essenceModel:         strings.TrimSpace(cfg.EssenceModel),
		essenceEscalateModel: EffectiveEssenceEscalateModel(cfg.EssenceModel, cfg.EssenceEscalateModel),
		essenceBatchSize:     cfg.EssenceBatchSize,
		tldrBatch:            cfg.TldrBatch,
		tldrModel:            strings.TrimSpace(cfg.TldrModel),
		clusterMinCosine:     cfg.ClusterMinCosine,
		quarantineTerms:      append([]string{}, cfg.QuarantineTerms...),
		attention:            cfg.Attention,
		attentionTrueMin:     cfg.AttentionTrueMin,
		essenceDumpDir:       strings.TrimSpace(cfg.EssenceDumpDir),
		essenceSystemPrompt:  strings.TrimSpace(cfg.EssenceSystemPrompt),
		tagCatalog:           cfg.TagCatalog,
		tagCurator:           cfg.TagCurator,
		now:                  now,
	}, nil
}

// Close closes the injected store.
func (r *Runner) Close() error {
	if r == nil || r.store == nil {
		return nil
	}
	return r.store.Close()
}

// Classify runs Operate for each item (sequential).
func (r *Runner) Classify(ctx context.Context, source string, items []Item) (Artifact, error) {
	const op = "mailreport.Runner.Classify"
	if r == nil {
		return Artifact{}, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if ctx == nil {
		return Artifact{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if r.store == nil {
		return Artifact{}, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	art := Artifact{
		Mode:           "report_only",
		PolypusBaseURL: r.polypusURL,
		JudgeModel:     r.judgeModel,
		AuthorModel:    r.authorModel,
		CatalogID:      r.vocab.ID,
		GeneratedAt:    r.now.Format(time.RFC3339),
		Source:         source,
		Messages:       []MessageRow{},
	}
	art.EmbedModel = r.embedModel
	art.AttachMinCosine = r.attachMinCosine
	art.WalkReinforceMin = r.walkReinforceMin
	art.EssenceDumpDir = r.essenceDumpDir
	art.EssenceEscalateModel = r.essenceEscalateModel
	r.appendResumeCompleteRows(ctx, items, &art)
	pending := r.pendingClassifyItems(ctx, items)
	if len(pending) > r.max {
		pending = pending[:r.max]
	}
	r.prefetchByHash = map[string]*harness.EssenceOut{}
	if r.h != nil && config.IsAttachStrategy(r.strategy) && len(pending) > 0 {
		ops := make([]harness.Op, 0, len(pending))
		for _, item := range pending {
			hash := strings.TrimSpace(item.ObjectHash)
			if hash == "" {
				continue
			}
			ops = append(ops, harness.Op{
				EssenceKey:   hash,
				WorldContext: r.operateWorld,
				Text:         formatItemText(item),
				Catalog:      r.cat,
			})
		}
		if err := r.h.PrefetchEssence(ctx, ops); err != nil {
			return art, sirerr.Wrap(err, sirerr.CodeFailed, op, "prefetch essence")
		}
		for _, op := range ops {
			if op.Essence != nil && strings.TrimSpace(op.EssenceKey) != "" {
				r.prefetchByHash[op.EssenceKey] = op.Essence
			}
		}
	}
	r.aiHops = 0
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		if err := r.classifyItem(ctx, item, &r.aiHops, &art); err != nil {
			return art, err
		}
	}
	return art, nil
}

// RunTagPass regenerates the tag register and runs tag curation after classify waves.
func (r *Runner) RunTagPass(ctx context.Context, art *Artifact) {
	if r == nil || art == nil {
		return
	}
	if art.NewlyClassified == 0 {
		art.TagPassSkipped = true
		return
	}
	if r.tagCurator != nil {
		curateRes, curErr := r.tagCurator.Run(ctx)
		if curErr != nil {
			art.TagCurateError = curErr.Error()
		} else {
			art.TagCurateDropped = curateRes.Dropped
			art.TagCurateKept = curateRes.Kept
			art.TagCurateMerged = curateRes.Merged
			if curateRes.RegisterPath != "" {
				art.TagRegisterPath = curateRes.RegisterPath
			}
			if curateRes.JudgeSkipped {
				art.TagCurateJevSkipped = true
			}
		}
		if cat := r.tagCurator.Catalog(); cat != nil {
			r.tagCatalog = cat
		}
		patchMessageTagShapes(&art.Messages, r.tagCatalog)
		return
	}
	if r.tagCatalog != nil {
		regPath, regErr := RegenerateTagRegister(ctx, r.store, r.tagCatalog)
		if regErr != nil {
			art.TagRegisterError = regErr.Error()
		} else {
			art.TagRegisterPath = regPath
		}
		patchMessageTagShapes(&art.Messages, r.tagCatalog)
	}
}

func (r *Runner) appendResumeCompleteRows(ctx context.Context, items []Item, art *Artifact) {
	for _, item := range items {
		hash := strings.TrimSpace(item.ObjectHash)
		if hash == "" {
			continue
		}
		got, getErr := r.store.Get(ctx, hash)
		if getErr != nil || !isResumeComplete(got) {
			continue
		}
		art.Messages = append(art.Messages, r.messageRowFromState(item, got))
		art.Classified++
	}
}

func patchMessageTagShapes(messages *[]MessageRow, cat *tags.Catalog) {
	if messages == nil || cat == nil {
		return
	}
	for i := range *messages {
		filtered := tags.FilterStoredShape((*messages)[i].Shape, cat)
		(*messages)[i].Shape = filtered
		if filtered == "" {
			(*messages)[i].Tags = nil
		} else {
			(*messages)[i].Tags = strings.Fields(filtered)
		}
	}
}

func (r *Runner) classifyItem(ctx context.Context, item Item, aiHops *int, art *Artifact) error {
	hash := strings.TrimSpace(item.ObjectHash)
	if hash == "" {
		art.Unresolved++
		return nil
	}
	got, getErr := r.store.Get(ctx, hash)
	if getErr == nil && isResumeComplete(got) {
		if !artifactHasHash(art, hash) {
			art.Messages = append(art.Messages, r.messageRowFromState(item, got))
			art.Classified++
		}
		return nil
	}
	if getErr != nil {
		if code, ok := sirerr.AsCode(getErr); !ok || code != sirerr.CodeNotFound {
			return getErr
		}
	}
	if *aiHops >= r.max {
		art.Omitted++
		return nil
	}
	token, skip, err := r.store.Claim(ctx, hash)
	if err != nil {
		return err
	}
	if skip {
		got, getErr := r.store.Get(ctx, hash)
		if getErr != nil {
			return getErr
		}
		if !artifactHasHash(art, hash) {
			art.Messages = append(art.Messages, r.messageRowFromState(item, got))
			art.Classified++
		}
		return nil
	}
	*aiHops++
	return r.operateAndPersist(ctx, hash, token, item, art)
}

func artifactHasHash(art *Artifact, hash string) bool {
	if art == nil {
		return false
	}
	for _, m := range art.Messages {
		if strings.TrimSpace(m.ObjectHash) == hash {
			return true
		}
	}
	return false
}

func isResumeComplete(got hoststore.StateRow) bool {
	return got.Status == "complete" && strings.TrimSpace(got.Error) == "" &&
		(strings.TrimSpace(got.TermID) != "" || len(got.Path) > 0)
}

func (r *Runner) operateAndPersist(ctx context.Context, hash, token string, item Item, art *Artifact) error {
	row := MessageRow{ObjectHash: hash, Subject: item.Subject, Sender: item.Sender}
	opCtx, cancel := context.WithTimeout(ctx, r.operateTimeout())
	defer cancel()
	res, opErr := r.runOperate(opCtx, item)
	if opErr != nil {
		if isUnavailable(opErr) {
			_ = r.store.Fail(ctx, hash, token, formatRowError(opErr))
			return opErr
		}
		row.Error = formatRowError(opErr)
		art.Messages = append(art.Messages, row)
		if failErr := r.store.Fail(ctx, hash, token, row.Error); failErr != nil {
			return failErr
		}
		return nil
	}
	fillMessageRowFromOperate(&row, r.strategy, res)
	r.applyAcceptedDraft(art, &row, res)
	if !assignmentComplete(row) {
		if row.Error == "" {
			row.Error = "empty assignment"
		}
		art.Messages = append(art.Messages, row)
		if failErr := r.store.Fail(ctx, hash, token, row.Error); failErr != nil {
			return failErr
		}
		return nil
	}
	emb, embErr := r.canonicalEmbedding(ctx, row)
	if embErr != nil {
		row.EmbedPersist = formatRowError(embErr)
	}
	if err := r.store.Complete(ctx, hash, token, stateFromRow(row, r.vocab.ID, r.judgeModel, r.embedModel), emb); err != nil {
		return err
	}
	art.Messages = append(art.Messages, row)
	art.Classified++
	art.NewlyClassified++
	return nil
}

func fillMessageRowFromOperate(row *MessageRow, fallbackStrategy string, res harness.Result) {
	row.Strategy = string(res.Strategy)
	if row.Strategy == "" {
		row.Strategy = fallbackStrategy
	}
	row.Kind = res.Kind
	row.About = res.About
	row.Shape = res.Shape
	row.Cosine = res.Cosine
	row.CanonicalTermID = res.CanonicalID
	row.Reinforced = res.Reinforced
	row.JudgeScore = res.JudgeScore
	row.Path = res.Path
	if len(res.Assigned) > 0 {
		row.TermID = res.Assigned[0].TermID
		row.Label = res.Assigned[0].Label
		row.Source = res.Assigned[0].Source
		if row.Source == "" {
			row.Source = sourceFromOperateResult(res)
		}
	}
	if res.Draft != nil {
		row.Draft = &DraftRecord{
			Kind:        res.Draft.Kind,
			ID:          res.Draft.ID,
			Parent:      res.Draft.Parent,
			Label:       res.Draft.Label,
			Description: res.Draft.Description,
			LeafID:      res.Draft.LeafID,
			Alias:       res.Draft.Alias,
		}
	}
}

func (r *Runner) applyAcceptedDraft(art *Artifact, row *MessageRow, res harness.Result) {
	if !res.DraftAccepted || !r.apply {
		return
	}
	termsBefore := len(r.vocab.Terms)
	vocab, applyErr := harness.Apply(r.vocab, res)
	if applyErr != nil {
		row.Error = formatRowError(applyErr)
		return
	}
	if err := SaveCatalog(r.catalogPath, vocab); err != nil {
		row.Error = formatRowError(err)
		return
	}
	r.vocab = vocab
	cat, buildErr := catalog.BuildCatalog(vocab)
	if buildErr != nil {
		row.Error = formatRowError(buildErr)
		return
	}
	r.cat = cat
	row.CatalogApplied = true
	if res.Draft == nil {
		return
	}
	switch res.Draft.Kind {
	case harness.DraftKindAlias:
		art.AliasesAdded++
	case harness.DraftKindNewLeaf, harness.DraftKindBreadcrumb:
		added := len(vocab.Terms) - termsBefore
		if added < 1 {
			added = 1
		}
		art.LeavesAdded += added
	}
}

func assignmentComplete(row MessageRow) bool {
	return strings.TrimSpace(row.TermID) != "" || len(row.Path) > 0
}

func (r *Runner) pendingClassifyItems(ctx context.Context, items []Item) []Item {
	complete := map[string]struct{}{}
	if hashes, err := r.store.ListCompleteHashes(ctx); err == nil {
		for _, h := range hashes {
			complete[h] = struct{}{}
		}
	}
	var pending []Item
	for _, item := range items {
		hash := strings.TrimSpace(item.ObjectHash)
		if hash == "" {
			continue
		}
		if _, ok := complete[hash]; ok {
			continue
		}
		pending = append(pending, item)
	}
	return pending
}

func (r *Runner) canonicalEmbedding(ctx context.Context, row MessageRow) (*hoststore.EmbeddingPut, error) {
	const op = "mailreport.Runner.canonicalEmbedding"
	if r == nil || r.seats.Embedder == nil || strings.TrimSpace(r.embedModel) == "" {
		return nil, nil
	}
	canonical := strings.TrimSpace(row.Kind + "\n" + row.About + "\n" + row.Shape)
	if canonical == "" {
		canonical = strings.TrimSpace(strings.Join(row.Path, " > ") + "\n" + row.Label + "\n" + row.TermID)
	}
	if canonical == "" {
		return nil, nil
	}
	embedCtx, span := startSeatSpan(ctx, op, r.embedModel)
	vecs, err := r.seats.Embedder.Embed(embedCtx, []string{canonical})
	if err != nil {
		wrapped := sirerr.Wrap(err, sirerr.CodeAI, op, "canonical embed").With("embed_persist", row.ObjectHash)
		endSeatSpan(span, wrapped)
		return nil, wrapped
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		empty := sirerr.New(sirerr.CodeFailed, op, "empty canonical embedding").With("embed_persist", row.ObjectHash)
		endSeatSpan(span, empty)
		return nil, empty
	}
	endSeatSpan(span, nil)
	return &hoststore.EmbeddingPut{ModelID: r.embedModel, Vec: vecs[0]}, nil
}

func (r *Runner) messageRowFromState(item Item, got hoststore.StateRow) MessageRow {
	tm := r.attentionTrueMin
	if tm == 0 {
		tm = defaultAttentionTrueMin
	}
	row := MessageRow{
		ObjectHash:      item.ObjectHash,
		Subject:         item.Subject,
		Sender:          item.Sender,
		TermID:          got.TermID,
		Path:            got.Path,
		Label:           got.Label,
		Source:          got.Source,
		Strategy:        got.Strategy,
		Kind:            got.Kind,
		About:           got.About,
		Shape:           got.Shape,
		Cosine:          got.Cosine,
		CanonicalTermID: got.CanonicalTermID,
		Reinforced:      got.Reinforced,
		JudgeScore:      got.JudgeScore,
		SenderTermID:    got.SenderTermID,
		SenderLabel:     got.SenderLabel,
		SenderMapsTo:    got.SenderMapsTo,
	}
	row.Attention = messageAttentionFromState(got, tm)
	if shape := strings.TrimSpace(got.Shape); shape != "" {
		row.Tags = strings.Fields(shape)
	}
	return row
}

func stateFromRow(row MessageRow, catalogID, classifyModel, embedModel string) hoststore.StateRow {
	return hoststore.StateRow{
		ObjectHash:      row.ObjectHash,
		TermID:          row.TermID,
		Path:            row.Path,
		Label:           row.Label,
		Source:          row.Source,
		Strategy:        row.Strategy,
		Kind:            row.Kind,
		About:           row.About,
		Shape:           row.Shape,
		Cosine:          row.Cosine,
		CanonicalTermID: row.CanonicalTermID,
		Reinforced:      row.Reinforced,
		JudgeScore:      row.JudgeScore,
		DraftKind:       draftKind(row.Draft),
		CatalogApplied:  row.CatalogApplied,
		SenderTermID:    row.SenderTermID,
		SenderLabel:     row.SenderLabel,
		SenderMapsTo:    row.SenderMapsTo,
		CatalogID:       catalogID,
		ClassifyModel:   classifyModel,
		EmbedModel:      embedModel,
	}
}

func (r *Runner) runOperate(ctx context.Context, item Item) (harness.Result, error) {
	const op = "mailreport.Runner.runOperate"
	if ctx != nil {
		ctx = withEssenceDumpID(ctx, item.ObjectHash)
	}
	text := formatItemText(item)
	hash := strings.TrimSpace(item.ObjectHash)
	opSpec := harness.Op{
		WorldContext: r.operateWorld,
		Text:         text,
		Catalog:      r.cat,
		EssenceKey:   hash,
	}
	if ess := r.prefetchByHash[hash]; ess != nil {
		opSpec.Essence = ess
	}
	var res harness.Result
	var opErr error
	if r.pipe == nil || r.pipe.Jobs == nil {
		r.seats.BindCatalog(r.cat)
		res, opErr = r.h.Operate(ctx, opSpec)
	} else {
		r.seats.BindCatalog(r.cat)
		r.pipe.SetPending(r.h, opSpec)
		_, genErr := r.pipe.Jobs.Generate(ctx, classifyGenerationConfig(), CreateClassifyInput(item.ObjectHash, text), nil)
		if genErr != nil {
			if isUnavailable(genErr) {
				return harness.Result{}, sirerr.Wrap(genErr, sirerr.CodeUnavailable, op, "job runner generate")
			}
			return harness.Result{}, sirerr.Wrap(genErr, sirerr.CodeFailed, op, "job runner generate")
		}
		res, opErr = r.pipe.TakeOutcome()
	}
	if opErr != nil {
		if !r.authorOnSkip && harness.CodeOf(opErr) == harness.CodeInvalidDraft {
			return harness.Result{}, nil
		}
		if isUnavailable(opErr) {
			return harness.Result{}, sirerr.Wrap(opErr, sirerr.CodeUnavailable, op, "operate")
		}
		return harness.Result{}, sirerr.Wrap(opErr, sirerr.CodeFailed, op, "operate")
	}
	return res, nil
}

func draftKind(d *DraftRecord) string {
	if d == nil {
		return ""
	}
	return d.Kind
}

func (r *Runner) operateTimeout() time.Duration {
	if config.IsAttachStrategy(r.strategy) {
		return operateTimeoutAttach
	}
	return operateTimeoutWalk
}

func sourceFromOperateResult(res harness.Result) string {
	if res.Draft != nil {
		switch res.Draft.Kind {
		case harness.DraftKindAlias:
			return "alias"
		case harness.DraftKindBreadcrumb:
			return "breadcrumb"
		}
	}
	return "judge"
}

func formatItemText(item Item) string {
	var b strings.Builder
	if s := strings.TrimSpace(item.Subject); s != "" {
		b.WriteString("Subject: ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	from := strings.TrimSpace(item.Sender)
	if from == "" {
		from = strings.TrimSpace(item.Headers.From)
	}
	if from != "" {
		b.WriteString("From: ")
		b.WriteString(from)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(strings.TrimSpace(item.Body))
	return strings.TrimSpace(b.String())
}

// CapItems sorts and caps a slice of items by object_hash for stable runs.
func CapItems(items []Item, max int) []Item {
	if max <= 0 || len(items) <= max {
		return items
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ObjectHash < items[j].ObjectHash
	})
	return items[:max]
}

func isUnavailable(err error) bool {
	for err != nil {
		if code, ok := sirerr.AsCode(err); ok && code == sirerr.CodeUnavailable {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// DedupeItemsByHash keeps first occurrence per object_hash.
func DedupeItemsByHash(items []Item) []Item {
	seen := map[string]struct{}{}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		h := strings.TrimSpace(it.ObjectHash)
		if h == "" {
			out = append(out, it)
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, it)
	}
	return out
}
