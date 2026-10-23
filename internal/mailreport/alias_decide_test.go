package mailreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

type captureAliasJudge struct {
	last harness.DecideIn
	out  harness.DecideOut
}

func (c *captureAliasJudge) Decide(_ context.Context, in harness.DecideIn) (harness.DecideOut, error) {
	c.last = in
	return c.out, nil
}

func aliasCollisionVocab() catalog.Vocabulary {
	return catalog.Vocabulary{
		ID: "demo",
		Terms: []catalog.Term{
			{ID: "notification", Label: "N", Description: "d"},
			{ID: "ci", Label: "CI", Parent: "notification", Description: "d"},
			{
				ID:          "ci-failure",
				Label:       "CI failure",
				Parent:      "ci",
				Description: "owner leaf",
				Aliases:     []string{"notification > ci > ci-failure"},
			},
			{
				ID:          "refactor",
				Label:       "Refactor",
				Parent:      "ci",
				Description: "challenger leaf",
			},
		},
	}
}

func testAliasRunner(t *testing.T, aliasJudge harness.Judge, minScore float64, catalogPath string) (*Runner, catalog.Vocabulary) {
	vocab := aliasCollisionVocab()
	cat, err := catalog.BuildCatalog(vocab)
	if err != nil {
		t.Fatal(err)
	}
	seats := Seats{Judge: &stubJudge{}, Author: &stubAuthor{}}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := CreatePipeline(PipelineConfig{Harness: h, Seats: seats})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := CreateRunner(RunnerConfig{
		Harness:       h,
		Seats:         seats,
		Pipeline:      pipe,
		CatalogPath:   catalogPath,
		ApplyCatalog:  true,
		Store:         testLedger(t),
		ClassifyMax:   10,
		AliasJudge:    aliasJudge,
		MinJudgeScore: minScore,
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	return runner, vocab
}

func TestApplyAcceptedDraft_aliasCollision_mercuryPicksOwner_retags(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/catalog.yaml"
	judge := &captureAliasJudge{
		out: harness.DecideOut{Choice: harness.ChoicePrefixUse + "refactor", Score: 0.95},
	}
	runner, _ := testAliasRunner(t, judge, 0, path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	art := &Artifact{}
	row := &MessageRow{TermID: "refactor", Label: "Refactor"}
	res := harness.Result{
		DraftAccepted: true,
		Draft: &harness.DraftOut{
			Kind:   harness.DraftKindAlias,
			LeafID: "refactor",
			Alias:  "notification > ci > ci-failure",
		},
	}
	runner.applyAcceptedDraft(ctx, art, row, res)
	if row.Error != "" {
		t.Fatalf("error: %q", row.Error)
	}
	if row.TermID != "refactor" {
		t.Fatalf("term: %q", row.TermID)
	}
	if !row.CatalogApplied {
		t.Fatal("expected catalog applied")
	}
	for _, term := range runner.vocab.Terms {
		if term.ID == "ci-failure" && len(term.Aliases) != 0 {
			t.Fatalf("loser still has alias: %v", term.Aliases)
		}
	}
}

func TestApplyAcceptedDraft_aliasCollision_skip_keepsChallenger_noError(t *testing.T) {
	path := t.TempDir() + "/catalog.yaml"
	judge := &captureAliasJudge{out: harness.DecideOut{Choice: harness.ChoiceSkip, Score: 1}}
	runner, vocabBefore := testAliasRunner(t, judge, 0, path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := &MessageRow{TermID: "refactor"}
	res := harness.Result{
		DraftAccepted: true,
		Draft: &harness.DraftOut{
			Kind:   harness.DraftKindAlias,
			LeafID: "refactor",
			Alias:  "notification > ci > ci-failure",
		},
	}
	runner.applyAcceptedDraft(ctx, &Artifact{}, row, res)
	if row.Error != "" {
		t.Fatalf("error: %q", row.Error)
	}
	if row.CatalogApplied {
		t.Fatal("expected no catalog write")
	}
	if row.TermID != "refactor" {
		t.Fatalf("term: %q", row.TermID)
	}
	if len(runner.vocab.Terms) != len(vocabBefore.Terms) {
		t.Fatal("vocab should be unchanged")
	}
}

func TestApplyAcceptedDraft_aliasCollision_nilJudge_softSkip(t *testing.T) {
	path := t.TempDir() + "/catalog.yaml"
	runner, _ := testAliasRunner(t, nil, 0, path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := &MessageRow{TermID: "refactor"}
	res := harness.Result{
		DraftAccepted: true,
		Draft: &harness.DraftOut{
			Kind:   harness.DraftKindAlias,
			LeafID: "refactor",
			Alias:  "notification > ci > ci-failure",
		},
	}
	runner.applyAcceptedDraft(ctx, &Artifact{}, row, res)
	if row.Error != "" {
		t.Fatalf("error: %q", row.Error)
	}
	if row.CatalogApplied {
		t.Fatal("expected no catalog write")
	}
}

func TestApplyAcceptedDraft_aliasCollision_lowScore_softSkip(t *testing.T) {
	path := t.TempDir() + "/catalog.yaml"
	judge := &captureAliasJudge{
		out: harness.DecideOut{Choice: harness.ChoicePrefixUse + "refactor", Score: 0.1},
	}
	runner, _ := testAliasRunner(t, judge, 0.8, path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := &MessageRow{TermID: "refactor"}
	res := harness.Result{
		DraftAccepted: true,
		Draft: &harness.DraftOut{
			Kind:   harness.DraftKindAlias,
			LeafID: "refactor",
			Alias:  "notification > ci > ci-failure",
		},
	}
	runner.applyAcceptedDraft(ctx, &Artifact{}, row, res)
	if row.Error != "" {
		t.Fatalf("error: %q", row.Error)
	}
	if row.CatalogApplied {
		t.Fatal("expected no catalog write on low score")
	}
}

func TestDecideAliasOwner_textOmitsMessagePII(t *testing.T) {
	vocab := aliasCollisionVocab()
	cat, err := catalog.BuildCatalog(vocab)
	if err != nil {
		t.Fatal(err)
	}
	judge := &captureAliasJudge{
		out: harness.DecideOut{Choice: harness.ChoicePrefixUse + "refactor", Score: 1},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, ok := decideAliasOwner(ctx, judge, cat, "notification > ci > ci-failure", "refactor", "ci-failure", 0)
	if !ok {
		t.Fatal("expected decision")
	}
	pii := []string{"Secret Invoice", "pii@example.com", "account 12345"}
	blob := judge.last.Text + judge.last.WorldContext
	for _, o := range judge.last.Options {
		blob += o.Description + o.Choice
	}
	for _, s := range pii {
		if strings.Contains(blob, s) {
			t.Fatalf("PII leaked into decide payload: %q in %q", s, blob)
		}
	}
	if !strings.HasPrefix(judge.last.Text, "KIND: ") {
		t.Fatalf("text: %q", judge.last.Text)
	}
}
