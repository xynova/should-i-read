package mailreport

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func testLedger(t *testing.T) *hoststore.Store {
	t.Helper()
	st, err := hoststore.CreateStore(hoststore.StoreConfig{
		Path: filepath.Join(t.TempDir(), "ledger.db"),
		Now:  time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

type stubEmbedder struct {
	vecs [][]float64
	err  error
}

func (s *stubEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	if s != nil && s.err != nil {
		return nil, s.err
	}
	if s == nil || len(s.vecs) == 0 {
		out := make([][]float64, len(texts))
		for i := range texts {
			out[i] = []float64{1, 0, 0}
		}
		return out, nil
	}
	return s.vecs, nil
}

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
	runner, err := CreateRunner(RunnerConfig{
		Harness:      h,
		Seats:        seats,
		Pipeline:     pipe,
		CatalogPath:  filepath.Join(t.TempDir(), "catalog.yaml"),
		ApplyCatalog: false,
		Store:        testLedger(t),
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

func TestRunner_Classify_unavailableFailsRow(t *testing.T) {
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
		Store:        testLedger(t),
		ClassifyMax:  10,
		PolypusURL:   "http://127.0.0.1:1320",
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	art, err := runner.Classify(ctx, SourceReport, []Item{
		{ObjectHash: "abc123", Subject: "CI failed", Sender: "noreply@github.com", Body: "Your workflow failed."},
		{ObjectHash: "def456", Subject: "CI failed again", Sender: "noreply@github.com", Body: "Still failed."},
	})
	if err != nil {
		t.Fatalf("classify should continue: %v", err)
	}
	if art.Classified != 0 {
		t.Fatalf("classified: %d", art.Classified)
	}
	if len(art.Messages) != 2 {
		t.Fatalf("messages: %d", len(art.Messages))
	}
	for i, row := range art.Messages {
		if row.Error == "" {
			t.Fatalf("message %d missing error", i)
		}
	}
}

func TestCreateRunnerNilStore(t *testing.T) {
	vocab := testInboxVocab()
	cat := testInboxCatalog(t)
	seats := Seats{Judge: &stubJudge{}, Author: &stubAuthor{}}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	_, err = CreateRunner(RunnerConfig{Harness: h, Seats: seats, ClassifyMax: 1}, vocab, cat)
	if err == nil {
		t.Fatal("expected nil store error")
	}
}

func TestRunner_Classify_skipsCompleteHash(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	vocab := testInboxVocab()
	cat := testInboxCatalog(t)
	judge := &stubJudge{
		choices: []string{
			harness.ChoicePrefixUse + "unwanted",
			harness.ChoicePrefixUse + "notification",
		},
		scores: []float64{0.9, 0.9},
	}
	seats := Seats{Judge: judge, Author: &stubAuthor{}}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := CreatePipeline(PipelineConfig{Harness: h, Seats: seats})
	if err != nil {
		t.Fatal(err)
	}
	store := testLedger(t)
	runner, err := CreateRunner(RunnerConfig{
		Harness: h, Seats: seats, Pipeline: pipe, Store: store, ClassifyMax: 10, PolypusURL: "http://127.0.0.1:1320",
		CatalogPath: filepath.Join(t.TempDir(), "c.yaml"),
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	item := Item{ObjectHash: "abc123", Subject: "CI failed", Sender: "n@x.com", Body: "fail"}
	if _, err := runner.Classify(ctx, SourceReport, []Item{item}); err != nil {
		t.Fatal(err)
	}
	first := judge.n
	if _, err := runner.Classify(ctx, SourceReport, []Item{item}); err != nil {
		t.Fatal(err)
	}
	if judge.n != first {
		t.Fatalf("Operate ran again: n=%d first=%d", judge.n, first)
	}
}

func TestRunner_Classify_persistsCanonicalEmbedding(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	vocab := testInboxVocab()
	cat := testInboxCatalog(t)
	seats := Seats{
		Judge: &stubJudge{
			choices: []string{
				harness.ChoicePrefixUse + "unwanted",
				harness.ChoicePrefixUse + "notification",
			},
			scores: []float64{0.9, 0.9},
		},
		Author:   &stubAuthor{},
		Embedder: &stubEmbedder{vecs: [][]float64{{1, 0, 0}}},
	}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := CreatePipeline(PipelineConfig{Harness: h, Seats: seats})
	if err != nil {
		t.Fatal(err)
	}
	store := testLedger(t)
	runner, err := CreateRunner(RunnerConfig{
		Harness: h, Seats: seats, Pipeline: pipe, Store: store, ClassifyMax: 10,
		PolypusURL: "http://127.0.0.1:1320", EmbedModel: "test-embed",
		CatalogPath: filepath.Join(t.TempDir(), "c.yaml"),
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := runner.Classify(ctx, SourceReport, []Item{{
		ObjectHash: "abc123", Subject: "CI failed", Sender: "n@x.com", Body: "fail",
	}}); err != nil {
		t.Fatal(err)
	}
	vec, err := store.GetEmbedding(ctx, "abc123", "test-embed")
	if err != nil || len(vec) != 3 {
		t.Fatalf("embedding: %v %v", vec, err)
	}
}

func TestRunner_Classify_embedPersistDoesNotFailComplete(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	vocab := testInboxVocab()
	cat := testInboxCatalog(t)
	seats := Seats{
		Judge: &stubJudge{
			choices: []string{
				harness.ChoicePrefixUse + "unwanted",
				harness.ChoicePrefixUse + "notification",
			},
			scores: []float64{0.9, 0.9},
		},
		Author:   &stubAuthor{},
		Embedder: &stubEmbedder{err: errors.New("polypus embed down")},
	}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := CreatePipeline(PipelineConfig{Harness: h, Seats: seats})
	if err != nil {
		t.Fatal(err)
	}
	store := testLedger(t)
	runner, err := CreateRunner(RunnerConfig{
		Harness: h, Seats: seats, Pipeline: pipe, Store: store, ClassifyMax: 10,
		PolypusURL: "http://127.0.0.1:1320", EmbedModel: "test-embed",
		CatalogPath: filepath.Join(t.TempDir(), "c.yaml"),
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	art, err := runner.Classify(ctx, SourceReport, []Item{{
		ObjectHash: "abc123", Subject: "CI failed", Sender: "n@x.com", Body: "fail",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if art.Classified != 1 {
		t.Fatalf("classified=%d", art.Classified)
	}
	if len(art.Messages) != 1 || strings.TrimSpace(art.Messages[0].EmbedPersist) == "" {
		t.Fatalf("embed_persist: %+v", art.Messages)
	}
	if art.Messages[0].Error != "" {
		t.Fatalf("classify error: %q", art.Messages[0].Error)
	}
	got, err := store.Get(ctx, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "complete" {
		t.Fatalf("status=%q", got.Status)
	}
	_, err = store.GetEmbedding(ctx, "abc123", "test-embed")
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeNotFound {
		t.Fatalf("want not_found embedding, got %v", err)
	}
}

func TestOperateTimeoutAttachIsFiveMinutes(t *testing.T) {
	t.Parallel()
	if operateTimeoutAttach != 5*time.Minute {
		t.Fatalf("attach operate timeout %s", operateTimeoutAttach)
	}
}
