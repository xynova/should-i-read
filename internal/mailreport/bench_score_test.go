package mailreport

import (
	"testing"
	"time"
)

func TestScoreBenchArtifact_handComputed(t *testing.T) {
	art := Artifact{
		Classified: 2,
		Omitted:    1,
		Unresolved: 0,
		Messages: []MessageRow{
			{ObjectHash: "h1", Kind: "a > b > payment-failure", About: "Cursor payment $66.00 failed", DescriptionCosine: 0.8},
			{ObjectHash: "h2", Kind: "a > b > other", About: "hello world", DescriptionCosine: 0.4, DescriptionCosineBelowMin: true},
			{ObjectHash: "h3", Error: "essence parse: invalid token"},
		},
	}
	labels := BenchLabelsFile{
		Revision: "rev",
		Labels: []BenchLabel{
			{ObjectHash: "h1", ExpectedLeaf: "payment-failure", RequiredTerms: []string{"cursor", "$66.00"}},
			{ObjectHash: "missing", ExpectedLeaf: "x", RequiredTerms: []string{"a"}},
		},
	}
	score := ScoreBenchArtifact(art, "baseline", "rev", labels, "ph", 0.55, map[string]time.Duration{"c1.json": 2 * time.Second}, 3*time.Second)
	if score.Failed != 1 {
		t.Fatalf("failed %d", score.Failed)
	}
	if score.CauseCounts["essence parse: invalid token"] != 1 {
		t.Fatalf("cause counts %v", score.CauseCounts)
	}
	if score.CosineMean < 0.59 || score.CosineMean > 0.61 {
		t.Fatalf("cosine mean %v", score.CosineMean)
	}
	if score.LeafAccuracy != 0.5 {
		t.Fatalf("leaf accuracy %v", score.LeafAccuracy)
	}
	wantRecall := (1.0 + 0.0) / 2.0
	if score.MeanTermRecall != wantRecall {
		t.Fatalf("term recall %v want %v", score.MeanTermRecall, wantRecall)
	}
	if score.ChunkRuntimesS["c1.json"] != 2.0 {
		t.Fatalf("runtime %v", score.ChunkRuntimesS)
	}
}
