package mailreport

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	operateTimeoutWalk   = 60 * time.Second
	operateTimeoutAttach = 180 * time.Second
)

// RunnerConfig configures classify runs.
type RunnerConfig struct {
	Harness          *harness.Harness
	Seats            Seats
	Pipeline         *Pipeline
	CatalogPath      string
	ApplyCatalog     bool
	ProgressPath     string
	ClassifyMax      int
	PolypusURL       string
	JudgeModel       string
	AuthorModel      string
	PreAI            bool
	AuthorOnSkip     bool
	SendersPath      string
	SendersVocab     catalog.Vocabulary
	SendersCat       *catalog.Catalog
	Strategy         string
	EmbedModel       string
	AttachMinCosine  float64
	WalkReinforceMin float64
	Now              time.Time
}

// Runner classifies items with taxonomy Operate.
type Runner struct {
	h                *harness.Harness
	seats            Seats
	pipe             *Pipeline
	catalogPath      string
	apply            bool
	progress         string
	max              int
	polypusURL       string
	judgeModel       string
	authorModel      string
	preAI            bool
	authorOnSkip     bool
	vocab            catalog.Vocabulary
	cat              *catalog.Catalog
	sendersPath      string
	sendersVocab     catalog.Vocabulary
	sendersCat       *catalog.Catalog
	strategy         string
	embedModel       string
	attachMinCosine  float64
	walkReinforceMin float64
	operateWorld     string
	now              time.Time
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
	if cfg.Pipeline != nil && cfg.Pipeline.Jobs == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "pipeline missing job runner")
	}
	max := cfg.ClassifyMax
	if max <= 0 {
		max = 50
	}
	strategy := strings.TrimSpace(cfg.Strategy)
	if strategy == "" {
		strategy = "walk"
	}
	world := WorldContext
	if strategy == "attach" {
		world = WorldContextAttach
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &Runner{
		h:                cfg.Harness,
		seats:            cfg.Seats,
		pipe:             cfg.Pipeline,
		catalogPath:      cfg.CatalogPath,
		apply:            cfg.ApplyCatalog,
		progress:         cfg.ProgressPath,
		max:              max,
		polypusURL:       cfg.PolypusURL,
		judgeModel:       strings.TrimSpace(cfg.JudgeModel),
		authorModel:      strings.TrimSpace(cfg.AuthorModel),
		embedModel:       strings.TrimSpace(cfg.EmbedModel),
		preAI:            cfg.PreAI,
		authorOnSkip:     cfg.AuthorOnSkip,
		vocab:            vocab,
		cat:              cat,
		sendersPath:      strings.TrimSpace(cfg.SendersPath),
		sendersVocab:     cfg.SendersVocab,
		sendersCat:       cfg.SendersCat,
		strategy:         strategy,
		attachMinCosine:  cfg.AttachMinCosine,
		walkReinforceMin: cfg.WalkReinforceMin,
		operateWorld:     world,
		now:              now,
	}, nil
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
	prog, err := LoadProgress(r.progress)
	if err != nil {
		return Artifact{}, err
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
	if r.sendersCat != nil {
		art.SendersCatalogID = strings.TrimSpace(r.sendersCat.Vocab.ID)
	}
	art.EmbedModel = r.embedModel
	art.AttachMinCosine = r.attachMinCosine
	art.WalkReinforceMin = r.walkReinforceMin
	aiHops := 0
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		hash := strings.TrimSpace(item.ObjectHash)
		if hash == "" {
			art.Unresolved++
			continue
		}
		if ent, ok := prog.Entries[hash]; ok && ent.Error == "" && progressComplete(ent) {
			art.Messages = append(art.Messages, MessageRow{
				ObjectHash:      hash,
				Subject:         item.Subject,
				Sender:          item.Sender,
				TermID:          ent.TermID,
				Path:            ent.Path,
				Label:           ent.Label,
				Source:          ent.Source,
				Strategy:        ent.Strategy,
				Kind:            ent.Kind,
				About:           ent.About,
				Shape:           ent.Shape,
				Cosine:          ent.Cosine,
				CanonicalTermID: ent.CanonicalTermID,
				Reinforced:      ent.Reinforced,
				JudgeScore:      ent.JudgeScore,
				SenderTermID:    ent.SenderTermID,
				SenderLabel:     ent.SenderLabel,
				SenderMapsTo:    ent.SenderMapsTo,
			})
			art.Classified++
			continue
		}
		fields := item.Headers.FieldMap(item.Sender)
		if r.preAI && r.strategy != "attach" {
			if pre, ok := HeuristicAssign(item.Headers, item.Sender, r.cat); ok {
				if err := r.recordPreAssign(&art, &prog, item, pre, fields); err != nil {
					return art, err
				}
				continue
			}
			if pre, ok := SenderCatalogAssign(r.sendersCat, r.cat, fields); ok {
				if err := r.recordPreAssign(&art, &prog, item, pre, fields); err != nil {
					return art, err
				}
				continue
			}
		}
		if aiHops >= r.max {
			art.Omitted++
			continue
		}
		aiHops++
		senderCatalogHit := false
		if r.preAI && r.sendersCat != nil {
			_, senderCatalogHit = SenderCatalogAssign(r.sendersCat, r.cat, fields)
		}
		row := MessageRow{ObjectHash: hash, Subject: item.Subject, Sender: item.Sender}
		opCtx, cancel := context.WithTimeout(ctx, r.operateTimeout())
		res, opErr := r.runOperate(opCtx, item)
		cancel()
		if opErr != nil {
			if isUnavailable(opErr) {
				return art, opErr
			}
			row.Error = formatRowError(opErr)
			stampSender(&row, r.sendersCat, fields)
			art.Messages = append(art.Messages, row)
			prog.Entries[hash] = progressEntryFromRow(row)
			if saveErr := SaveProgress(r.progress, prog); saveErr != nil {
				return art, saveErr
			}
			continue
		}
		row.Strategy = string(res.Strategy)
		if row.Strategy == "" {
			row.Strategy = r.strategy
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
		if res.DraftAccepted && r.apply {
			termsBefore := len(r.vocab.Terms)
			vocab, applyErr := harness.Apply(r.vocab, res)
			if applyErr != nil {
				row.Error = formatRowError(applyErr)
			} else {
				if err := SaveCatalog(r.catalogPath, vocab); err != nil {
					row.Error = formatRowError(err)
				} else {
					r.vocab = vocab
					cat, buildErr := catalog.BuildCatalog(vocab)
					if buildErr != nil {
						row.Error = formatRowError(buildErr)
					} else {
						r.cat = cat
						row.CatalogApplied = true
						if res.Draft != nil {
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
					}
				}
			}
		}
		if r.apply && !senderCatalogHit && row.TermID != "" && ShouldLearnSenders(row.TermID) {
			if learnErr := r.learnSenders(row.TermID, fields); learnErr != nil {
				row.Error = formatRowError(learnErr)
			}
		}
		stampSender(&row, r.sendersCat, fields)
		art.Messages = append(art.Messages, row)
		art.Classified++
		prog.Entries[hash] = progressEntryFromRow(row)
		if err := SaveProgress(r.progress, prog); err != nil {
			return art, err
		}
	}
	return art, nil
}

func progressComplete(ent ProgressEntry) bool {
	if ent.TermID != "" || len(ent.Path) > 0 {
		return true
	}
	return ent.Source == SourceHeuristic || ent.Source == SourceSenderCatalog
}

func (r *Runner) recordPreAssign(art *Artifact, prog *ProgressFile, item Item, pre PreAssign, fields map[string]string) error {
	hash := strings.TrimSpace(item.ObjectHash)
	row := MessageRow{
		ObjectHash: hash,
		Subject:    item.Subject,
		Sender:     item.Sender,
		TermID:     pre.TermID,
		Path:       pre.Path,
		Label:      pre.Label,
		Source:     pre.Source,
	}
	stampSender(&row, r.sendersCat, fields)
	art.Messages = append(art.Messages, row)
	art.Classified++
	prog.Entries[hash] = progressEntryFromRow(row)
	return SaveProgress(r.progress, *prog)
}

func (r *Runner) learnSenders(inboxLeaf string, fields map[string]string) error {
	const op = "mailreport.Runner.learnSenders"
	if r.sendersPath == "" || r.sendersCat == nil {
		return nil
	}
	learnFields := LearnFieldsForSenders(fields)
	if len(learnFields) == 0 {
		return nil
	}
	termID := MintSenderTermID(learnFields["from"])
	if termID == "" {
		return nil
	}
	if _, ok := r.sendersCat.Lookup(termID); !ok {
		r.sendersVocab.Terms = append(r.sendersVocab.Terms, catalog.Term{
			ID:     termID,
			Label:  termID,
			MapsTo: inboxLeaf,
		})
	}
	out, err := catalog.LearnExact(r.sendersVocab, termID, learnFields)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "learn exact")
	}
	if err := SaveCatalog(r.sendersPath, out); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "save senders catalog")
	}
	cat, err := catalog.BuildCatalog(out)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "build senders catalog")
	}
	r.sendersVocab = out
	r.sendersCat = cat
	return nil
}

func (r *Runner) runOperate(ctx context.Context, item Item) (harness.Result, error) {
	const op = "mailreport.Runner.runOperate"
	text := formatItemText(item)
	opSpec := harness.Op{
		WorldContext: r.operateWorld,
		Text:         text,
		Catalog:      r.cat,
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
	if r != nil && r.strategy == "attach" {
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
