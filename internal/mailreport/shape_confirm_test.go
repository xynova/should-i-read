package mailreport

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
)

func testKindVocab() catalog.Vocabulary {
	return catalog.Vocabulary{
		ID:    "inbox-kind",
		Label: "Inbox kind",
		Terms: []catalog.Term{
			{ID: "pipeline-failure", Label: "Pipeline failure", Description: "CI pipeline failed"},
		},
	}
}

func testShapeRunner(t *testing.T, store *hoststore.Store, judge harness.Judge) *Runner {
	t.Helper()
	vocab := testKindVocab()
	cat, err := catalog.BuildCatalog(vocab)
	if err != nil {
		t.Fatal(err)
	}
	seats := Seats{Judge: judge, Author: &stubAuthor{}, Essencer: stubEssencer{}, Embedder: &stubEmbedder{}}
	h, err := harness.CreateHarness(harness.Config{
		Strategy: harness.StrategyAttach,
		Judge:    judge,
		Author:   seats.Author,
		Essencer: seats.Essencer,
		Embedder: seats.Embedder,
	})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := CreateRunner(RunnerConfig{
		Harness:            h,
		Seats:              seats,
		CatalogPath:        filepath.Join(t.TempDir(), "kind.yaml"),
		Store:              store,
		ClassifyMax:        10,
		Strategy:           string(harness.StrategyAttach),
		ShapeConfirm:       true,
		ShapeMinRepeats:    3,
		ShapeConfirmMin:    0.90,
		ShapeAllowedLeaves: []string{"pipeline-failure"},
		EmbedModel:         "embed-model",
		EssenceModel:       "essence-model",
		PolypusURL:         "http://127.0.0.1:1320",
	}, vocab, cat)
	if err != nil {
		t.Fatal(err)
	}
	runner.seats.Embedder = &stubEmbedder{}
	return runner
}

func TestShapePromoteAfterEssenceComplete(t *testing.T) {
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runner := testShapeRunner(t, store, &stubJudge{})
	fpEssence := essenceFingerprint(runner.essenceModel, runner.essenceEscalateModel, DefaultEssenceSystemPrompt())
	art := &Artifact{}
	subject := "Acme | Failed pipeline for main | deadbeef"
	sender := "GitLab <notify@gitlab.com>"
	for i := 1; i <= 3; i++ {
		hash := fmt.Sprintf("obj-%d", i)
		if err := store.PutEssence(ctx, hash, fpEssence, hoststore.EssenceRow{
			Kind:         "notification > ci > pipeline-failure",
			EssenceModel: runner.essenceModel,
			PromptHash:   fpEssence.PromptHash,
		}); err != nil {
			t.Fatal(err)
		}
		item := Item{ObjectHash: hash, Subject: subject, Sender: sender}
		row := MessageRow{
			Kind:     "notification > ci > pipeline-failure",
			TermID:   "pipeline-failure",
			Strategy: string(harness.StrategyAttach),
		}
		runner.maybePromoteShape(ctx, item, row, art)
	}
	if art.ShapesPromoted != 1 {
		t.Fatalf("ShapesPromoted=%d", art.ShapesPromoted)
	}
	row, ok, err := store.GetShape(ctx, ShapeFingerprint(senderDomain(sender), runner.subjectTemplater.Template(subject)))
	if err != nil || !ok || row.Status != hoststore.ShapeStatusActive {
		t.Fatalf("shape ok=%v status=%q err=%v", ok, row.Status, err)
	}
}

func TestShapePromoteSkipsStalePromptHash(t *testing.T) {
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runner := testShapeRunner(t, store, &stubJudge{})
	staleFP := essenceFingerprint(runner.essenceModel, runner.essenceEscalateModel, DefaultEssenceSystemPrompt())
	staleFP.PromptHash = "stale"
	if err := store.PutEssence(ctx, "obj-stale", staleFP, hoststore.EssenceRow{
		Kind:         "notification > ci > pipeline-failure",
		EssenceModel: runner.essenceModel,
		PromptHash:   "stale",
	}); err != nil {
		t.Fatal(err)
	}
	art := &Artifact{}
	item := Item{ObjectHash: "obj-stale", Subject: "Failed pipeline | abcdef01", Sender: "notify@gitlab.com"}
	row := MessageRow{Kind: "notification > ci > pipeline-failure", TermID: "pipeline-failure"}
	runner.maybePromoteShape(ctx, item, row, art)
	if art.ShapesPromoted != 0 {
		t.Fatalf("ShapesPromoted=%d", art.ShapesPromoted)
	}
}

func TestShapeConfirmSkipJevMatch(t *testing.T) {
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	judge := &stubJudge{choices: []string{shapeChoiceMatch}, scores: []float64{0.95}}
	runner := testShapeRunner(t, store, judge)
	subject := "Failed pipeline for branch | cafebabe"
	sender := "GitLab <notify@gitlab.com>"
	domain := senderDomain(sender)
	template := runner.subjectTemplater.Template(subject)
	fp := ShapeFingerprint(domain, template)
	_, err := store.RecordShapeHit(ctx, hoststore.ShapeHit{
		Fingerprint:    fp,
		Domain:         domain,
		Template:       template,
		Kind:           "notification > ci > pipeline-failure",
		TermID:         "pipeline-failure",
		LastSlot:       "pipeline-failure",
		PromptHash:     runner.currentEssencePromptHash(),
		SeedObjectHash: "seed-1",
		MinRepeats:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	art, err := runner.Classify(ctx, SourceReport, []Item{{
		ObjectHash: "new-mail",
		Subject:    subject,
		Sender:     sender,
		Body:       "pipeline failed",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if art.ShapeConfirmed != 1 || art.Classified != 1 {
		t.Fatalf("ShapeConfirmed=%d Classified=%d", art.ShapeConfirmed, art.Classified)
	}
	if art.Messages[0].Strategy != StrategyShapeConfirm {
		t.Fatalf("strategy=%q", art.Messages[0].Strategy)
	}
}

type stubEssencer struct{}

func (stubEssencer) Essence(_ context.Context, _ harness.EssenceIn) (harness.EssenceOut, error) {
	return harness.EssenceOut{
		Kind:  "notification > ci > pipeline-failure",
		About: "pipeline notification",
		Shape: "ci",
	}, nil
}

func TestShapeConfirmDisabledOnBenchSource(t *testing.T) {
	runner := testShapeRunner(t, testLedger(t), &stubJudge{})
	if runner.shapeConfirmEnabled(SourceBench) {
		t.Fatal("bench must disable shape confirm")
	}
}
