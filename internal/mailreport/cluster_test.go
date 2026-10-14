package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestAssignClustersSameCanonicalTerm(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rows := []MessageRow{
		{ObjectHash: "h1", CanonicalTermID: "changelog", Kind: "a > b > changelog"},
		{ObjectHash: "h2", CanonicalTermID: "changelog", Kind: "x > y > changelog"},
	}
	out, clusters, err := AssignClusters(ctx, rows, AssignOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 || clusters[0].Size != 2 {
		t.Fatalf("clusters: %+v", clusters)
	}
	if out[0].ClusterID != out[1].ClusterID {
		t.Fatalf("ids: %s %s", out[0].ClusterID, out[1].ClusterID)
	}
}

func TestAssignClustersSeparatesBuildFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rows := []MessageRow{
		{ObjectHash: "h1", CanonicalTermID: "changelog"},
		{ObjectHash: "h2", CanonicalTermID: "build-failure"},
	}
	_, clusters, err := AssignClusters(ctx, rows, AssignOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 2 {
		t.Fatalf("clusters: %d", len(clusters))
	}
}

func TestAssignClustersSenderDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rows := []MessageRow{
		{ObjectHash: "h1", Sender: "Bot <noreply@github.com>"},
		{ObjectHash: "h2", Sender: "Other <x@github.com>"},
	}
	_, clusters, err := AssignClusters(ctx, rows, AssignOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 || clusters[0].SenderDomain != "github.com" {
		t.Fatalf("clusters: %+v", clusters)
	}
}

func TestAssignClustersSkipsErrorRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rows := []MessageRow{
		{ObjectHash: "h1", Error: "fail"},
		{ObjectHash: "h2", CanonicalTermID: "changelog"},
	}
	out, clusters, err := AssignClusters(ctx, rows, AssignOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 || out[0].ClusterID != "" {
		t.Fatalf("out: %+v clusters: %+v", out, clusters)
	}
}

func TestAssignClustersUnlabeledSingleton(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rows := []MessageRow{{ObjectHash: "solo"}}
	_, clusters, err := AssignClusters(ctx, rows, AssignOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 || clusters[0].Size != 1 {
		t.Fatalf("clusters: %+v", clusters)
	}
}

func TestAssignClustersSimilarAttachInArtifactOnly(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	rows := []MessageRow{
		{ObjectHash: "term1", CanonicalTermID: "changelog"},
		{ObjectHash: "orphan", Sender: "x@example.com"},
	}
	similar := func(_ context.Context, hash, _ string, _ int) ([]hoststore.Neighbor, error) {
		if hash == "orphan" {
			return []hoststore.Neighbor{
				{ObjectHash: "outside", Cosine: 0.99},
				{ObjectHash: "term1", Cosine: 0.85},
			}, nil
		}
		return nil, sirerr.New(sirerr.CodeNotFound, "test", "missing")
	}
	_, clusters, err := AssignClusters(ctx, rows, AssignOpts{
		MinCosine:  0.8,
		EmbedModel: "embed",
		Similar:    similar,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 || clusters[0].Size != 2 {
		t.Fatalf("clusters: %+v", clusters)
	}
}

func TestAssignClustersRequiresDeadlineWhenSimilar(t *testing.T) {
	t.Parallel()
	similar := func(context.Context, string, string, int) ([]hoststore.Neighbor, error) {
		return nil, nil
	}
	_, _, err := AssignClusters(context.Background(), []MessageRow{{ObjectHash: "h1"}}, AssignOpts{
		Similar: similar,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func TestClusterFingerprintChangesWithMembers(t *testing.T) {
	t.Parallel()
	gk := "term:changelog"
	fp2 := clusterFingerprint(gk, []string{"a", "b"})
	fp3 := clusterFingerprint(gk, []string{"a", "b", "c"})
	if fp2 == fp3 {
		t.Fatal("fingerprint should change when members change")
	}
}

func TestSenderDomain(t *testing.T) {
	t.Parallel()
	if senderDomain("GitHub <notifications@github.com>") != "github.com" {
		t.Fatal("parse address")
	}
}
