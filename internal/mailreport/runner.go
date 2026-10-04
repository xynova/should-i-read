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

const operateTimeout = 60 * time.Second

// RunnerConfig configures classify runs.
type RunnerConfig struct {
	Harness      *harness.Harness
	Seats        Seats
	Pipeline     *Pipeline
	CatalogPath  string
	ApplyCatalog bool
	ProgressPath string
	ClassifyMax  int
	PolypusURL   string
	JudgeModel   string
	AuthorModel  string
}

// Runner classifies items with taxonomy Operate.
type Runner struct {
	h           *harness.Harness
	seats       Seats
	pipe        *Pipeline
	catalogPath string
	apply       bool
	progress    string
	max         int
	polypusURL  string
	judgeModel  string
	authorModel string
	vocab       catalog.Vocabulary
	cat         *catalog.Catalog
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
	return &Runner{
		h:           cfg.Harness,
		seats:       cfg.Seats,
		pipe:        cfg.Pipeline,
		catalogPath: cfg.CatalogPath,
		apply:       cfg.ApplyCatalog,
		progress:    cfg.ProgressPath,
		max:         max,
		polypusURL:  cfg.PolypusURL,
		judgeModel:  strings.TrimSpace(cfg.JudgeModel),
		authorModel: strings.TrimSpace(cfg.AuthorModel),
		vocab:       vocab,
		cat:         cat,
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
	omitted := 0
	if len(items) > r.max {
		omitted = len(items) - r.max
		items = items[:r.max]
	}
	art := Artifact{
		Mode:           "report_only",
		PolypusBaseURL: r.polypusURL,
		JudgeModel:     r.judgeModel,
		AuthorModel:    r.authorModel,
		CatalogID:      r.vocab.ID,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Source:         source,
		Omitted:        omitted,
		Messages:       []MessageRow{},
	}
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		hash := strings.TrimSpace(item.ObjectHash)
		if hash == "" {
			art.Unresolved++
			continue
		}
		if ent, ok := prog.Entries[hash]; ok && ent.TermID != "" && ent.Error == "" {
			art.Messages = append(art.Messages, MessageRow{
				ObjectHash: hash,
				Subject:    item.Subject,
				Sender:     item.Sender,
				TermID:     ent.TermID,
				Path:       ent.Path,
				Label:      ent.Label,
				JudgeScore: ent.JudgeScore,
			})
			art.Classified++
			continue
		}
		row := MessageRow{ObjectHash: hash, Subject: item.Subject, Sender: item.Sender}
		opCtx, cancel := context.WithTimeout(ctx, operateTimeout)
		res, opErr := r.runOperate(opCtx, item)
		cancel()
		if opErr != nil {
			if isUnavailable(opErr) {
				return art, opErr
			}
			row.Error = opErr.Error()
			art.Messages = append(art.Messages, row)
			prog.Entries[hash] = ProgressEntry{Error: row.Error, UpdatedAt: progressNow()}
			if saveErr := SaveProgress(r.progress, prog); saveErr != nil {
				return art, saveErr
			}
			continue
		}
		row.JudgeScore = res.JudgeScore
		row.Path = res.Path
		if len(res.Assigned) > 0 {
			row.TermID = res.Assigned[0].TermID
			row.Label = res.Assigned[0].Label
			row.Source = res.Assigned[0].Source
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
			vocab, applyErr := harness.Apply(r.vocab, res)
			if applyErr != nil {
				row.Error = applyErr.Error()
			} else {
				if err := SaveCatalog(r.catalogPath, vocab); err != nil {
					row.Error = err.Error()
				} else {
					r.vocab = vocab
					cat, buildErr := catalog.BuildCatalog(vocab)
					if buildErr != nil {
						row.Error = buildErr.Error()
					} else {
						r.cat = cat
						row.CatalogApplied = true
						if res.Draft != nil {
							switch res.Draft.Kind {
							case harness.DraftKindAlias:
								art.AliasesAdded++
							case harness.DraftKindNewLeaf:
								art.LeavesAdded++
							}
						}
					}
				}
			}
		}
		art.Messages = append(art.Messages, row)
		art.Classified++
		prog.Entries[hash] = ProgressEntry{
			TermID:         row.TermID,
			Path:           row.Path,
			Label:          row.Label,
			JudgeScore:     row.JudgeScore,
			DraftKind:      draftKind(row.Draft),
			CatalogApplied: row.CatalogApplied,
			Error:          row.Error,
			UpdatedAt:      progressNow(),
		}
		if err := SaveProgress(r.progress, prog); err != nil {
			return art, err
		}
	}
	return art, nil
}

func (r *Runner) runOperate(ctx context.Context, item Item) (harness.Result, error) {
	const op = "mailreport.Runner.runOperate"
	text := formatItemText(item)
	opSpec := harness.Op{
		WorldContext: WorldContext,
		Text:         text,
		Catalog:      r.cat,
	}
	if r.pipe == nil || r.pipe.Jobs == nil {
		r.seats.BindCatalog(r.cat)
		return r.h.Operate(ctx, opSpec)
	}
	r.seats.BindCatalog(r.cat)
	r.pipe.SetPending(r.h, opSpec)
	_, genErr := r.pipe.Jobs.Generate(ctx, classifyGenerationConfig(), CreateClassifyInput(item.ObjectHash, text), nil)
	if genErr != nil {
		if isUnavailable(genErr) {
			return harness.Result{}, sirerr.Wrap(genErr, sirerr.CodeUnavailable, op, "job runner generate")
		}
		return harness.Result{}, sirerr.Wrap(genErr, sirerr.CodeFailed, op, "job runner generate")
	}
	return r.pipe.TakeOutcome()
}

func draftKind(d *DraftRecord) string {
	if d == nil {
		return ""
	}
	return d.Kind
}

func formatItemText(item Item) string {
	var b strings.Builder
	if s := strings.TrimSpace(item.Subject); s != "" {
		b.WriteString("Subject: ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	from := strings.TrimSpace(item.Sender)
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
