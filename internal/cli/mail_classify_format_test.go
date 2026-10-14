package cli

import (
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/mailreport"
)

func TestFormatClassifyArtifactClusters(t *testing.T) {
	art := mailreport.Artifact{
		Classified: 2,
		Clusters: []mailreport.ClusterRow{
			{
				ID:              "c-abc",
				TermID:          "changelog",
				Size:            2,
				TLDR:            "Two Fabric release notes.",
				SuggestedAction: mailreport.SuggestedReview,
			},
		},
	}
	out := formatClassifyArtifact(art)
	if !strings.Contains(out, "cluster") {
		t.Fatalf("missing cluster: %s", out)
	}
	if !strings.Contains(out, "changelog (2) review:") {
		t.Fatalf("missing cluster line: %s", out)
	}
	if !strings.Contains(out, "Two Fabric release notes.") {
		t.Fatalf("missing tldr: %s", out)
	}
}

func TestFormatClassifyArtifactAttentionSort(t *testing.T) {
	art := mailreport.Artifact{
		Classified: 2,
		Clusters: []mailreport.ClusterRow{
			{
				ID:              "c-low",
				TermID:          "failure",
				Size:            11,
				TLDR:            "pipelines",
				SuggestedAction: mailreport.SuggestedQuarantineCandidate,
				Attention:       &mailreport.Attention{Urgency: 0.45, InvolvesDecision: true},
			},
			{
				ID:              "c-high",
				TermID:          "reminder",
				Size:            1,
				TLDR:            "quota",
				SuggestedAction: mailreport.SuggestedReview,
				Attention:       &mailreport.Attention{Urgency: 0.82, InvolvesDecision: true},
			},
		},
	}
	mailreport.SortClustersByAttention(art.Clusters)
	out := formatClassifyArtifact(art)
	idxHigh := strings.Index(out, "urg=0.82")
	idxLow := strings.Index(out, "failure (11)")
	if idxHigh < 0 || idxLow < 0 {
		t.Fatalf("missing urgency lines: %s", out)
	}
	if idxLow < idxHigh {
		t.Fatalf("expected higher urgency first: %s", out)
	}
	if !strings.Contains(out, "decision=yes") {
		t.Fatalf("missing decision: %s", out)
	}
}

func TestFormatClassifyArtifactEssenceDump(t *testing.T) {
	art := mailreport.Artifact{
		Classified:     0,
		EssenceDumpDir: "tmp/essence-parse",
		Messages: []mailreport.MessageRow{
			{
				ObjectHash: "h1",
				Subject:    "Failed pipeline",
				Error:      "[essence] mailreport.polypusEssencer.Essence: essence parse after retries: about or shape empty",
			},
		},
	}
	out := formatClassifyArtifact(art)
	if !strings.Contains(out, "essence dumps tmp/essence-parse") {
		t.Fatalf("missing dump line: %s", out)
	}
}
