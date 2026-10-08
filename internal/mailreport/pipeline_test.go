package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
)

func TestPipeline_Generate_operateLLM_noExtraHTTP(t *testing.T) {
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
	if pipe.Jobs == nil {
		t.Fatal("expected job runner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = core.WithExecutionState(ctx)
	text := "Subject: CI\n\nfailed"
	pipe.SetPending(h, harness.Op{
		WorldContext: WorldContext,
		Text:         text,
		Catalog:      cat,
	})
	outs, err := pipe.Jobs.Generate(ctx, classifyGenerationConfig(), CreateClassifyInput("hash1", text), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, opErr := pipe.TakeOutcome()
	if opErr != nil {
		t.Fatal(opErr)
	}
	if len(res.Assigned) == 0 || res.Assigned[0].TermID != "notification" {
		t.Fatalf("assigned: %+v", res.Assigned)
	}
	if outs[fieldTermID] != "notification" {
		t.Fatalf("outputs term_id: %v full %v", outs[fieldTermID], outs)
	}
}
