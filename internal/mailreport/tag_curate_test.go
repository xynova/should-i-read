package mailreport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"github.com/behaviorengineering/taxonomy/pkg/tags"

	"github.com/xynova/should-i-read/internal/hoststore"
)

type stubTagJudge struct {
	hop1 map[string]harness.DecideOut
	hop2 map[string]harness.DecideOut
}

func (s *stubTagJudge) Decide(_ context.Context, in harness.DecideIn) (harness.DecideOut, error) {
	hop2 := len(in.Options) > 2
	for tok, dec := range s.hop1 {
		if !strings.Contains(in.Text, "Token: "+tok) {
			continue
		}
		if hop2 {
			if d, ok := s.hop2[tok]; ok {
				return d, nil
			}
			return harness.DecideOut{Choice: "keep", Score: 0.9}, nil
		}
		return dec, nil
	}
	return harness.DecideOut{Choice: "keep", Score: 0.5}, nil
}

func testTagCurateCatalog(t *testing.T) (*tags.Catalog, string, string) {
	t.Helper()
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.txt")
	if err := os.WriteFile(seed, []byte("fyi\npromotional\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dep := filepath.Join(dir, "mail-tag-deprecations.toml")
	reg := filepath.Join(dir, "register.txt")
	cat, err := tags.CreateCatalog(MailTagCatalogConfig(reg, dep, seed))
	if err != nil {
		t.Fatal(err)
	}
	return cat, dep, seed
}

func TestTagCurator_deterministicUnify(t *testing.T) {
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, _, err := store.Claim(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	shape := "a with rectangular notification fyi"
	if err := store.Complete(ctx, "h1", tok, hoststore.StateRow{TermID: "t", Shape: shape}, nil); err != nil {
		t.Fatal(err)
	}
	cat, _, _ := testTagCurateCatalog(t)
	curator, err := CreateTagCurator(TagCuratorConfig{
		Store:   store,
		Catalog: cat,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := curator.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rewrote != 1 {
		t.Fatalf("rewrote %d", res.Rewrote)
	}
	got, err := store.Get(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	want := "fyi notification"
	if got.Shape != want {
		t.Fatalf("shape %q want %q", got.Shape, want)
	}
}

func TestTagCurator_jevDropAndMerge(t *testing.T) {
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, _, err := store.Claim(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, "h1", tok, hoststore.StateRow{TermID: "t", Shape: "notification fyi promo"}, nil); err != nil {
		t.Fatal(err)
	}
	cat, dep, _ := testTagCurateCatalog(t)
	judge := &stubTagJudge{
		hop1: map[string]harness.DecideOut{
			"notification": {Choice: "drop", Score: 0.9},
			"promo":        {Choice: "keep", Score: 0.9},
		},
		hop2: map[string]harness.DecideOut{
			"promo": {Choice: "promotional", Score: 0.9},
		},
	}
	curator, err := CreateTagCurator(TagCuratorConfig{
		Store:    store,
		Catalog:  cat,
		MinScore: 0.8,
		Judge:    judge,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := curator.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Shape != "fyi promotional" {
		t.Fatalf("shape %q", got.Shape)
	}
	if len(res.Dropped) != 1 || res.Dropped[0] != "notification" {
		t.Fatalf("dropped %v", res.Dropped)
	}
	if res.Merged["promo"] != "promotional" {
		t.Fatalf("merged %v", res.Merged)
	}
	b, err := os.ReadFile(dep)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "[[dropped]]") || !strings.Contains(body, "notification") {
		t.Fatalf("toml %q", body)
	}
	if !strings.Contains(body, "from = \"promo\"") {
		t.Fatalf("toml %q", body)
	}
}

func TestTagCurator_belowThresholdNoTOML(t *testing.T) {
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, _, err := store.Claim(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, "h1", tok, hoststore.StateRow{TermID: "t", Shape: "notification"}, nil); err != nil {
		t.Fatal(err)
	}
	cat, dep, _ := testTagCurateCatalog(t)
	judge := &stubTagJudge{hop1: map[string]harness.DecideOut{
		"notification": {Choice: "drop", Score: 0.2},
	}}
	curator, err := CreateTagCurator(TagCuratorConfig{
		Store:    store,
		Catalog:  cat,
		MinScore: 0.8,
		Judge:    judge,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = curator.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dep)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "id = \"notification\"") {
		t.Fatalf("should not write dropped entry: %s", b)
	}
}

func TestRunner_Classify_tagCurateErrorDoesNotFail(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, _, err := store.Claim(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, "h1", tok, hoststore.StateRow{TermID: "t", Shape: "notification"}, nil); err != nil {
		t.Fatal(err)
	}
	cat, _, _ := testTagCurateCatalog(t)
	vocab := testInboxVocab()
	taxCat, err := catalog.BuildCatalog(vocab)
	if err != nil {
		t.Fatal(err)
	}
	seats := Seats{
		Judge:  &stubJudge{choices: []string{harness.ChoiceReject}, scores: []float64{1}},
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
	failCurator, err := CreateTagCurator(TagCuratorConfig{
		Store:   store,
		Catalog: cat,
		Judge:   &errTagJudge{},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := CreateRunner(RunnerConfig{
		Harness:     h,
		Seats:       seats,
		Pipeline:    pipe,
		Store:       store,
		ClassifyMax: 0,
		TagCatalog:  cat,
		TagCurator:  failCurator,
		Now:         time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}, vocab, taxCat)
	if err != nil {
		t.Fatal(err)
	}
	art, err := runner.Classify(ctx, SourceReport, nil)
	if err != nil {
		t.Fatalf("Classify should not fail on curate: %v", err)
	}
	runner.RunTagPass(ctx, &art)
	if art.TagCurateError == "" {
		t.Fatal("expected tag curate error on artifact")
	}
}

type errTagJudge struct{}

func (e *errTagJudge) Decide(context.Context, harness.DecideIn) (harness.DecideOut, error) {
	return harness.DecideOut{}, errors.New("jev failed")
}

func TestMailTagObjective_hopCopy(t *testing.T) {
	obj := mailTagObjective()
	if obj.WorldContext != tagCurateWorld {
		t.Fatalf("world %q", obj.WorldContext)
	}
	if obj.Purpose != tagKeepDescription || obj.NotPurpose != tagDropDescription {
		t.Fatalf("purpose/not %+v", obj)
	}
	if obj.TokenHint != tagTokenUseLine || obj.KeepOwn != tagKeepOwnDescription {
		t.Fatalf("hint/keep %+v", obj)
	}
	if !strings.Contains(obj.NotPurpose, "email anatomy") {
		t.Fatalf("drop %q", obj.NotPurpose)
	}
}

type captureTagJudge struct {
	last harness.DecideIn
}

func (c *captureTagJudge) Decide(_ context.Context, in harness.DecideIn) (harness.DecideOut, error) {
	c.last = in
	return harness.DecideOut{Choice: "drop", Score: 0.9}, nil
}

func TestTagCurator_passesScopeToJudge(t *testing.T) {
	store := testLedger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, _, err := store.Claim(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, "h1", tok, hoststore.StateRow{TermID: "t", Shape: "notification"}, nil); err != nil {
		t.Fatal(err)
	}
	cat, _, _ := testTagCurateCatalog(t)
	capJudge := &captureTagJudge{}
	curator, err := CreateTagCurator(TagCuratorConfig{
		Store:    store,
		Catalog:  cat,
		MinScore: 0.8,
		Judge:    capJudge,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := curator.Run(ctx); err != nil {
		t.Fatal(err)
	}
	in := capJudge.last
	if in.WorldContext != tagCurateWorld {
		t.Fatalf("world %q", in.WorldContext)
	}
	if !strings.Contains(in.Text, tagTokenUseLine) {
		t.Fatalf("text %q", in.Text)
	}
	if len(in.Options) != 2 || in.Options[0].Description == "" || in.Options[1].Description == "" {
		t.Fatalf("options %+v", in.Options)
	}
}
