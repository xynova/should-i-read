package mailreport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/behaviorengineering/strop/pkg/jev"
	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const classifyRowID = "classify-row"

type polypusJudge struct {
	jev   *jev.Client
	model string
}

// JudgeBatchItem is one row for batched SystemOne judge calls.
type JudgeBatchItem struct {
	RowID string
	In    harness.DecideIn
}

type polypusAuthor struct {
	client *polypus.Client
	model  string
	cat    *catalog.Catalog
}

// BindCatalog updates seat prompts for the current catalog (call before Operate).
func (s Seats) BindCatalog(cat *catalog.Catalog) {
	if a, ok := s.Author.(*polypusAuthor); ok {
		a.cat = cat
	}
}

type authorJSON struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Parent      string `json:"parent"`
	Label       string `json:"label"`
	Description string `json:"description"`
	LeafID      string `json:"leafId"`
	Alias       string `json:"alias"`
}

type polypusEssencer struct {
	client *polypus.Client
	model  string
}

func (e *polypusEssencer) Essence(ctx context.Context, in harness.EssenceIn) (harness.EssenceOut, error) {
	const op = "mailreport.polypusEssencer.Essence"
	if e == nil || e.client == nil {
		return harness.EssenceOut{}, sirerr.New(sirerr.CodeInvalid, op, "nil essencer")
	}
	ctx, span := startSeatSpan(ctx, "mailreport.Essence", e.model)
	var out harness.EssenceOut
	var retErr error
	defer func() { endSeatSpan(span, retErr) }()
	messageText := AuthorMessageBody(in.Text)
	prompt := strings.TrimSpace(in.WorldContext) + "\n\nMessage:\n" + messageText
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		content, err := e.client.Chat(ctx, polypus.ChatRequest{
			Model:       e.model,
			Temperature: 0,
			Messages: []polypus.ChatMessage{
				{Role: polypus.RoleSystem, Content: essenceSystemPrompt},
				{Role: polypus.RoleUser, Content: prompt},
			},
		})
		if err != nil {
			code := sirerr.CodeAI
			if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeUnavailable {
				code = sirerr.CodeUnavailable
			}
			retErr = sirerr.Wrap(err, code, op, "essence chat")
			return harness.EssenceOut{}, retErr
		}
		parsed, err := parseEssenceResponse(content)
		if err == nil {
			out = parsed
			return out, nil
		}
		lastErr = err
	}
	retErr = sirerr.Wrap(lastErr, sirerr.CodeAI, op, "essence parse after retries")
	return harness.EssenceOut{}, retErr
}

// SeatsConfig wires Polypus models for taxonomy seats.
type SeatsConfig struct {
	Client       *polypus.Client
	JudgeModel   string
	AuthorModel  string
	EssenceModel string
	Embedder     harness.Embedder
	JEVClient    *jev.Client
}

// CreateSeats wires Polypus SystemOne Judge and chat Author (optional Essencer/Embedder).
func CreateSeats(cfg SeatsConfig) (Seats, error) {
	const op = "mailreport.CreateSeats"
	if cfg.Client == nil {
		return Seats{}, sirerr.New(sirerr.CodeInvalid, op, "nil polypus client")
	}
	judgeModel := strings.TrimSpace(cfg.JudgeModel)
	authorModel := strings.TrimSpace(cfg.AuthorModel)
	essenceModel := strings.TrimSpace(cfg.EssenceModel)
	if judgeModel == "" {
		return Seats{}, sirerr.New(sirerr.CodeInvalid, op, "judge model id is empty")
	}
	if authorModel == "" {
		return Seats{}, sirerr.New(sirerr.CodeInvalid, op, "author model id is empty")
	}
	jevClient := cfg.JEVClient
	if jevClient == nil {
		jevClient = jev.NewClient(cfg.Client.BaseURL, cfg.Client.HTTPClient, "")
	}
	out := Seats{
		Judge:  &polypusJudge{jev: jevClient, model: judgeModel},
		Author: &polypusAuthor{client: cfg.Client, model: authorModel},
	}
	if essenceModel != "" {
		out.Essencer = &polypusEssencer{client: cfg.Client, model: essenceModel}
	}
	out.Embedder = cfg.Embedder
	return out, nil
}

func (j *polypusJudge) Decide(ctx context.Context, in harness.DecideIn) (harness.DecideOut, error) {
	const op = "mailreport.polypusJudge.Decide"
	outs, err := j.BatchDecide(ctx, []JudgeBatchItem{{RowID: classifyRowID, In: in}})
	if err != nil {
		return harness.DecideOut{}, err
	}
	if len(outs) == 0 {
		return harness.DecideOut{}, sirerr.New(sirerr.CodeFailed, op, "empty batch result")
	}
	return outs[0], nil
}

// BatchDecide scores many DecideIn values, batching SystemOne when option sets match.
func (j *polypusJudge) BatchDecide(ctx context.Context, items []JudgeBatchItem) ([]harness.DecideOut, error) {
	const op = "mailreport.polypusJudge.BatchDecide"
	if j == nil || j.jev == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil judge")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
	}
	if len(items) == 0 {
		return nil, nil
	}
	rows := make([]jev.Row, 0, len(items))
	rowMeta := make([]judgeBatchMeta, 0, len(items))
	for _, it := range items {
		rowID := strings.TrimSpace(it.RowID)
		if rowID == "" {
			rowID = classifyRowID
		}
		in := it.In
		if len(in.Options) == 0 {
			return nil, sirerr.New(sirerr.CodeInvalid, op, "no packed options")
		}
		allowed, criteria := packedCriteria(in.Options)
		questions := noulQuestions(in.Options, criteria)
		rows = append(rows, jev.Row{
			ID:          rowID,
			StateSuffix: strings.TrimSpace(in.Text),
			Questions:   questions,
		})
		rowMeta = append(rowMeta, judgeBatchMeta{
			rowID:      rowID,
			options:    in.Options,
			allowed:    allowed,
			preferSkip: !isGateOptions(in.Options),
		})
	}
	world := strings.TrimSpace(items[0].In.WorldContext)
	reqs, err := jev.BatchRows(rows, world, jev.DefaultMaxQuestions)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeAI, op, "batch rows")
	}
	answersByRow := map[string]map[string]jev.Answer{}
	for _, req := range reqs {
		req.Model = j.model
		raw, err := j.jev.Eval(ctx, req)
		if err != nil {
			if strings.Contains(err.Error(), "unavailable") {
				return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "jev eval")
			}
			return nil, sirerr.Wrap(err, sirerr.CodeAI, op, "jev eval")
		}
		for key, ans := range raw {
			rowID, choice, serr := jev.SplitKey(key)
			if serr != nil {
				continue
			}
			m := answersByRow[rowID]
			if m == nil {
				m = map[string]jev.Answer{}
				answersByRow[rowID] = m
			}
			m[choice] = ans
		}
	}
	outs := make([]harness.DecideOut, 0, len(rowMeta))
	for _, meta := range rowMeta {
		sys := jevAnswersToSystemOne(answersByRow[meta.rowID], meta.options)
		dec, err := decideFromNoul(op, meta.options, meta.allowed, sys, meta.preferSkip)
		if err != nil {
			return nil, err
		}
		outs = append(outs, dec)
	}
	return outs, nil
}

type judgeBatchMeta struct {
	rowID      string
	options    []harness.PackedOption
	allowed    map[string]struct{}
	preferSkip bool
}

func packedCriteria(options []harness.PackedOption) (map[string]struct{}, map[string]string) {
	allowed := map[string]struct{}{}
	criteria := map[string]string{}
	for _, o := range options {
		allowed[o.Choice] = struct{}{}
		label := strings.TrimSpace(o.Label)
		if label == "" {
			label = o.Choice
		}
		desc := strings.TrimSpace(o.Description)
		if desc == "" {
			criteria[o.Choice] = label
		} else {
			criteria[o.Choice] = label + ": " + desc
		}
	}
	return allowed, criteria
}

func noulQuestions(options []harness.PackedOption, criteria map[string]string) map[string]jev.Question {
	questions := make(map[string]jev.Question, len(options))
	for _, o := range options {
		questions[o.Choice] = jev.Question{
			Type:         "noul",
			Instructions: "Score 0 to 1 how honestly this option fits the message. " + criteria[o.Choice],
		}
	}
	return questions
}

func jevAnswersToSystemOne(raw map[string]jev.Answer, options []harness.PackedOption) polypus.SystemOneResponse {
	mapped := make(map[string]polypus.SystemOneAnswer, len(options))
	for _, o := range options {
		if a, ok := raw[o.Choice]; ok {
			mapped[o.Choice] = polypus.SystemOneAnswer{Type: a.Type, Noul: a.Noul, Choice: a.Choice}
		}
	}
	return polypus.SystemOneResponse{Answers: mapped}
}

func isGateOptions(options []harness.PackedOption) bool {
	for _, o := range options {
		if o.Choice == harness.ChoiceAcceptDraft || o.Choice == harness.ChoiceReject {
			return true
		}
	}
	return false
}

func decideFromNoul(op string, options []harness.PackedOption, allowed map[string]struct{}, out polypus.SystemOneResponse, preferSkipOnTie bool) (harness.DecideOut, error) {
	best := ""
	bestScore := -1.0
	tied := 0
	skipTied := false
	rejectTied := false
	for _, o := range options {
		ans, ok := out.Answers[o.Choice]
		if !ok {
			continue
		}
		if ans.Noul > bestScore {
			bestScore = ans.Noul
			best = o.Choice
			tied = 1
			skipTied = o.Choice == harness.ChoiceSkip
			rejectTied = o.Choice == harness.ChoiceReject
			continue
		}
		if ans.Noul != bestScore {
			continue
		}
		tied++
		if o.Choice == harness.ChoiceSkip {
			skipTied = true
		}
		if o.Choice == harness.ChoiceReject {
			rejectTied = true
		}
	}
	if best == "" {
		return harness.DecideOut{}, sirerr.New(sirerr.CodeFailed, op, "missing noul answers")
	}
	if preferSkipOnTie {
		if _, ok := allowed[harness.ChoiceSkip]; ok && (skipTied || tied > 1) {
			best = harness.ChoiceSkip
		}
	} else if rejectTied {
		best = harness.ChoiceReject
	}
	if _, ok := allowed[best]; !ok {
		return harness.DecideOut{}, sirerr.New(sirerr.CodeFailed, op, "choice not in packed options").With("choice", best)
	}
	return harness.DecideOut{Choice: best, Score: bestScore}, nil
}

func (a *polypusAuthor) Draft(ctx context.Context, in harness.DraftIn) (harness.DraftOut, error) {
	const op = "mailreport.polypusAuthor.Draft"
	leafLines := formatAuthorLeavesScoped(a.cat, in.Parents)
	messageText := AuthorMessageBody(in.Text)
	prompt := buildAuthorPrompt(in, leafLines, messageText)
	content, err := a.client.Chat(ctx, polypus.ChatRequest{
		Model:       a.model,
		Temperature: 0,
		Messages: []polypus.ChatMessage{
			{Role: polypus.RoleSystem, Content: authorSystemPrompt},
			{Role: polypus.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		code := sirerr.CodeAI
		if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeUnavailable {
			code = sirerr.CodeUnavailable
		}
		return harness.DraftOut{}, sirerr.Wrap(err, code, op, "author chat").
			With("chat_model", a.model)
	}
	var parsed authorJSON
	if err := decodeJSONContent(content, &parsed); err != nil {
		return harness.DraftOut{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "parse author json")
	}
	if err := validateAuthorDraft(a.cat, in.Parents, parsed); err != nil {
		return harness.DraftOut{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "validate author draft")
	}
	return harness.DraftOut{
		Kind:        strings.TrimSpace(parsed.Kind),
		ID:          strings.TrimSpace(parsed.ID),
		Parent:      strings.TrimSpace(parsed.Parent),
		Label:       strings.TrimSpace(parsed.Label),
		Description: strings.TrimSpace(parsed.Description),
		LeafID:      strings.TrimSpace(parsed.LeafID),
		Alias:       strings.TrimSpace(parsed.Alias),
	}, nil
}

func decodeJSONContent(content string, dest any) error {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	return json.Unmarshal([]byte(content), dest)
}

func formatAuthorLeavesScoped(cat *catalog.Catalog, parents []harness.PackedOption) string {
	if cat == nil {
		return ""
	}
	if len(parents) == 0 {
		return formatAuthorLeaves(cat)
	}
	seen := map[string]struct{}{}
	var b strings.Builder
	for _, p := range parents {
		root := strings.TrimSpace(p.ParentID)
		if root == "" {
			continue
		}
		for _, leafID := range cat.DescendantLeaves(root) {
			if _, dup := seen[leafID]; dup {
				continue
			}
			seen[leafID] = struct{}{}
			rt, ok := cat.Lookup(leafID)
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "- id=%s label=%s description=%s\n", rt.ID, rt.Label, rt.Description)
		}
	}
	return b.String()
}

func formatAuthorLeaves(cat *catalog.Catalog) string {
	if cat == nil {
		return ""
	}
	var b strings.Builder
	for _, rt := range cat.LeafTerms() {
		fmt.Fprintf(&b, "- id=%s label=%s description=%s\n", rt.ID, rt.Label, rt.Description)
	}
	return b.String()
}
