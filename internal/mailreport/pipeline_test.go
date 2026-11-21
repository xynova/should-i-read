package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

func TestOperate_stubJudgeAuthor(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	text := "Subject: CI\n\nfailed"
	res, err := h.Operate(ctx, harness.Op{
		WorldContext: WorldContext,
		Text:         text,
		Catalog:      cat,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Assigned) == 0 || res.Assigned[0].TermID != "notification" {
		t.Fatalf("assigned: %+v", res.Assigned)
	}
}
