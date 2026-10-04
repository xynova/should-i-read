package mailreport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type polypusJudge struct {
	client *polypus.Client
	model  string
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

// CreateSeats wires Polypus SystemOne Judge and chat Author.
func CreateSeats(client *polypus.Client, judgeModel, authorModel string) (Seats, error) {
	const op = "mailreport.CreateSeats"
	if client == nil {
		return Seats{}, sirerr.New(sirerr.CodeInvalid, op, "nil polypus client")
	}
	judgeModel = strings.TrimSpace(judgeModel)
	authorModel = strings.TrimSpace(authorModel)
	if judgeModel == "" {
		return Seats{}, sirerr.New(sirerr.CodeInvalid, op, "judge model id is empty")
	}
	if authorModel == "" {
		return Seats{}, sirerr.New(sirerr.CodeInvalid, op, "author model id is empty")
	}
	return Seats{
		Judge:  &polypusJudge{client: client, model: judgeModel},
		Author: &polypusAuthor{client: client, model: authorModel},
	}, nil
}

const maxJudgeNoulQuestions = 64

func (j *polypusJudge) Decide(ctx context.Context, in harness.DecideIn) (harness.DecideOut, error) {
	const op = "mailreport.polypusJudge.Decide"
	if j == nil || j.client == nil {
		return harness.DecideOut{}, sirerr.New(sirerr.CodeInvalid, op, "nil judge")
	}
	if ctx == nil {
		return harness.DecideOut{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if len(in.Options) == 0 {
		return harness.DecideOut{}, sirerr.New(sirerr.CodeInvalid, op, "no packed options")
	}
	allowed := map[string]struct{}{}
	criteria := map[string]string{}
	for _, o := range in.Options {
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
	state := strings.TrimSpace(in.WorldContext) + "\n\n" + strings.TrimSpace(in.Text)
	out, err := j.systemOneNoulPerOption(ctx, state, in.Options, criteria)
	if err != nil {
		if isUnavailable(err) {
			return harness.DecideOut{}, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "judge systemone")
		}
		return harness.DecideOut{}, sirerr.Wrap(err, sirerr.CodeAI, op, "judge systemone")
	}
	preferSkip := !isGateOptions(in.Options)
	return decideFromNoul(op, in.Options, allowed, out, preferSkip)
}

func isGateOptions(options []harness.PackedOption) bool {
	for _, o := range options {
		if o.Choice == harness.ChoiceAcceptDraft || o.Choice == harness.ChoiceReject {
			return true
		}
	}
	return false
}

func (j *polypusJudge) systemOneNoulPerOption(ctx context.Context, state string, options []harness.PackedOption, criteria map[string]string) (polypus.SystemOneResponse, error) {
	const op = "mailreport.polypusJudge.systemOneNoulPerOption"
	if len(options) > maxJudgeNoulQuestions {
		return polypus.SystemOneResponse{}, sirerr.New(sirerr.CodeAI, op, "too many packed options")
	}
	questions := make(map[string]polypus.SystemOneQuestion, len(options))
	for _, o := range options {
		qid := systemOneQuestionID(o.Choice)
		questions[qid] = polypus.SystemOneQuestion{
			Type:         "noul",
			Instructions: "Score 0 to 1 how honestly this option fits the message. " + criteria[o.Choice],
		}
	}
	req := polypus.SystemOneRequest{
		Model:     j.model,
		State:     state,
		Questions: questions,
	}
	out, err := j.client.SystemOne(ctx, req)
	if err != nil {
		return polypus.SystemOneResponse{}, err
	}
	return remapSystemOneAnswers(out, options), nil
}

// systemOneQuestionID maps packed choice ids to safe question keys (colons in use: ids).
func systemOneQuestionID(choice string) string {
	return strings.ReplaceAll(choice, ":", "__")
}

func remapSystemOneAnswers(out polypus.SystemOneResponse, options []harness.PackedOption) polypus.SystemOneResponse {
	if len(out.Answers) == 0 {
		return out
	}
	mapped := make(map[string]polypus.SystemOneAnswer, len(out.Answers))
	for _, o := range options {
		qid := systemOneQuestionID(o.Choice)
		if ans, ok := out.Answers[qid]; ok {
			mapped[o.Choice] = ans
			continue
		}
		if ans, ok := out.Answers[o.Choice]; ok {
			mapped[o.Choice] = ans
		}
	}
	out.Answers = mapped
	return out
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
	prompt := buildAuthorPrompt(in, formatAuthorLeavesScoped(a.cat, in.Parents))
	content, err := a.client.Chat(ctx, polypus.ChatRequest{
		Model:       a.model,
		Temperature: 0,
		Messages: []polypus.ChatMessage{
			{Role: "system", Content: "Reply with JSON only. Prefer kind alias when an existing leaf fits. kind is alias or new_leaf. For alias include leafId and alias. For new_leaf include id (kebab-case), parent (branch id), label, description."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return harness.DraftOut{}, sirerr.Wrap(err, sirerr.CodeAI, op, "author chat")
	}
	var parsed authorJSON
	if err := decodeJSONContent(content, &parsed); err != nil {
		return harness.DraftOut{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "parse author json")
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

func buildAuthorPrompt(in harness.DraftIn, leafLines string) string {
	var b strings.Builder
	b.WriteString("World context:\n")
	b.WriteString(in.WorldContext)
	b.WriteString("\n\nMessage:\n")
	b.WriteString(in.Text)
	b.WriteString("\n\nReason for author:\n")
	b.WriteString(in.Reason)
	b.WriteString("\n\nExisting leaves (prefer alias onto one of these):\n")
	b.WriteString(leafLines)
	b.WriteString("\n\nBranches:\n")
	for _, p := range in.Parents {
		fmt.Fprintf(&b, "- parent=%s label=%s\n", p.ParentID, p.Label)
	}
	return b.String()
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
