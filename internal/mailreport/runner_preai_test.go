package mailreport

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/pimdir"
)

func newPreAIRunner(t *testing.T, judge *stubJudge, senders *catalog.Catalog) *Runner {
	t.Setenv("NO_COLOR", "1")
	vocab := testInboxVocab()
	cat, err := catalog.BuildCatalog(vocab)
	if err != nil {
		t.Fatal(err)
	}
	seats := Seats{Judge: judge, Author: NoopAuthor()}
	h, err := harness.CreateHarness(harness.Config{Judge: seats.Judge, Author: seats.Author})
	if err != nil {
		t.Fatal(err)
	}
	progress := filepath.Join(t.TempDir(), "progress.json")
	runner, err := CreateRunner(RunnerConfig{
		Harness:      h,
		Seats:        seats,
		CatalogPath:  filepath.Join(t.TempDir(), "catalog.yaml"),
		ApplyCatalog: false,
		ProgressPath: progress,
		ClassifyMax:  0,
		PreAI:        true,
		AuthorOnSkip: false,
		SendersCat:   senders,
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func TestRunner_Classify_heuristicNoOperate(t *testing.T) {
	senders := testSendersGitHubCatalog(t)
	judge := &stubJudge{err: nil}
	runner := newPreAIRunner(t, judge, senders)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	art, err := runner.Classify(ctx, SourceReport, []Item{{
		ObjectHash: "hash1",
		Subject:    "CI failed",
		Sender:     "notifications@github.com",
		Headers:    pimdir.MailHeaders{From: "notifications@github.com"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if art.Classified != 1 || art.SendersCatalogID != "senders" {
		t.Fatalf("art: classified=%d senders_catalog=%q", art.Classified, art.SendersCatalogID)
	}
	m := art.Messages[0]
	if m.TermID != "notification" || m.Source != SourceHeuristic {
		t.Fatalf("inbox: %+v", m)
	}
	if m.SenderTermID != "sender-github-notifications" || m.SenderMapsTo != "notification" {
		t.Fatalf("sender stamp: %+v", m)
	}
	if judge.n != 0 {
		t.Fatalf("judge called %d times", judge.n)
	}
}

func TestRunner_Classify_senderCatalogOnly(t *testing.T) {
	senders, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "senders",
		Terms: []catalog.Term{{
			ID: "sender-alerts", Label: "Alerts", MapsTo: "notification",
			Patterns: []catalog.FieldPattern{{Field: "from", Re: "^alerts@example\\.com$"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	judge := &stubJudge{err: nil}
	runner := newPreAIRunner(t, judge, senders)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	art, err := runner.Classify(ctx, SourceReport, []Item{{
		ObjectHash: "hash-alerts",
		Subject:    "Disk full",
		Sender:     "alerts@example.com",
		Headers:    pimdir.MailHeaders{From: "alerts@example.com"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	m := art.Messages[0]
	if m.Source != SourceSenderCatalog || m.TermID != "notification" {
		t.Fatalf("inbox: %+v", m)
	}
	if m.SenderTermID != "sender-alerts" {
		t.Fatalf("sender stamp: %+v", m)
	}
	if judge.n != 0 {
		t.Fatalf("judge called %d times", judge.n)
	}
}

func TestRunner_Classify_progressReusesSenderFields(t *testing.T) {
	senders := testSendersGitHubCatalog(t)
	judge := &stubJudge{err: nil}
	runner := newPreAIRunner(t, judge, senders)
	item := Item{
		ObjectHash: "hash-reuse",
		Subject:    "CI failed",
		Sender:     "notifications@github.com",
		Headers:    pimdir.MailHeaders{From: "notifications@github.com"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := runner.Classify(ctx, SourceReport, []Item{item}); err != nil {
		t.Fatal(err)
	}
	art, err := runner.Classify(ctx, SourceReport, []Item{item})
	if err != nil {
		t.Fatal(err)
	}
	m := art.Messages[0]
	if m.SenderTermID != "sender-github-notifications" {
		t.Fatalf("reuse sender: %+v", m)
	}
	if judge.n != 0 {
		t.Fatalf("judge called %d times", judge.n)
	}
}
