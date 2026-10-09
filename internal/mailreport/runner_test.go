package mailreport

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

type stubJudge struct {
	choices []string
	scores  []float64
	n       int
	err     error
}

func (s *stubJudge) Decide(_ context.Context, _ harness.DecideIn) (harness.DecideOut, error) {
	if s.err != nil {
		return harness.DecideOut{}, s.err
	}
	i := s.n
	s.n++
	if i >= len(s.choices) {
		return harness.DecideOut{Choice: harness.ChoiceReject, Score: 1}, nil
	}
	return harness.DecideOut{Choice: s.choices[i], Score: s.scores[i]}, nil
}

type stubAuthor struct {
	draft harness.DraftOut
}

func (s *stubAuthor) Draft(_ context.Context, _ harness.DraftIn) (harness.DraftOut, error) {
	return s.draft, nil
}

func testInboxCatalog(t *testing.T) *catalog.Catalog {
	cat, err := catalog.BuildCatalog(testInboxVocab())
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func testInboxVocab() catalog.Vocabulary {
	return catalog.Vocabulary{
		ID:    "inbox-mail",
		Label: "Inbox",
		Terms: []catalog.Term{
			{ID: "keep", Label: "Keep", Description: "keep branch"},
			{ID: "unwanted", Label: "Unwanted", Description: "unwanted branch"},
			{ID: "notification", Label: "Notification", Parent: "unwanted", Description: "service alerts"},
		},
	}
}

func TestRunner_Classify_useLeaf(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	vocab := testInboxVocab()
	cat, err := catalog.BuildCatalog(vocab)
	if err != nil {
		t.Fatal(err)
	}
	seats := Seats{
		Judge: &stubJudge{
			choices: []string{
				harness.ChoicePrefixUse + "unwanted",
				harness.ChoicePrefixUse + "notification",
			},
			scores: []float64{0.9, 0.9},
		},
		Author: &stubAuthor{},
	}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := CreatePipeline(PipelineConfig{Harness: h, Seats: seats})
	if err != nil {
		t.Fatal(err)
	}
	progress := filepath.Join(t.TempDir(), "progress.json")
	runner, err := CreateRunner(RunnerConfig{
		Harness:      h,
		Seats:        seats,
		Pipeline:     pipe,
		CatalogPath:  filepath.Join(t.TempDir(), "catalog.yaml"),
		ApplyCatalog: false,
		ProgressPath: progress,
		ClassifyMax:  10,
		PolypusURL:   "http://127.0.0.1:1320",
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	art, err := runner.Classify(ctx, SourceReport, []Item{{
		ObjectHash: "abc123",
		Subject:    "CI failed",
		Sender:     "noreply@github.com",
		Body:       "Your workflow failed.",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if art.Classified != 1 {
		t.Fatalf("classified: %d", art.Classified)
	}
	if art.Messages[0].TermID != "notification" {
		t.Fatalf("term: %+v", art.Messages[0])
	}
}

func TestRunner_Classify_unavailableAborts(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	vocab := testInboxVocab()
	cat, err := catalog.BuildCatalog(vocab)
	if err != nil {
		t.Fatal(err)
	}
	seats := Seats{
		Judge:  &stubJudge{err: sirerr.New(sirerr.CodeUnavailable, "test", "polypus down")},
		Author: &stubAuthor{},
	}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := CreatePipeline(PipelineConfig{Harness: h, Seats: seats})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := CreateRunner(RunnerConfig{
		Harness:      h,
		Seats:        seats,
		Pipeline:     pipe,
		CatalogPath:  filepath.Join(t.TempDir(), "catalog.yaml"),
		ApplyCatalog: false,
		ProgressPath: filepath.Join(t.TempDir(), "progress.json"),
		ClassifyMax:  10,
		PolypusURL:   "http://127.0.0.1:1320",
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = runner.Classify(ctx, SourceReport, []Item{{
		ObjectHash: "abc123",
		Subject:    "CI failed",
		Sender:     "noreply@github.com",
		Body:       "Your workflow failed.",
	}})
	if err == nil {
		t.Fatal("expected unavailable")
	}
	if !isUnavailable(err) {
		t.Fatalf("want unavailable through Operate: %v", err)
	}
}
