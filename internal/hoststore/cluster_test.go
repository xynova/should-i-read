package hoststore

import (
	"context"
	"testing"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func sampleCluster() ClusterRecord {
	return ClusterRecord{
		ClusterID:       "c-abc123",
		Fingerprint:     "fp-one",
		TermID:          "changelog",
		Size:            2,
		TLDR:            "Two release notes.",
		SuggestedAction: SuggestedReview,
		TLDRModel:       "model-a",
		MemberHashes:    []string{"hash-a", "hash-b"},
	}
}

func TestPutClusterGetByFingerprint(t *testing.T) {
	st, ctx := testStore(t)
	rec := sampleCluster()
	if err := st.PutCluster(ctx, rec); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetClusterByFingerprint(ctx, rec.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if got.TLDR != rec.TLDR || got.ClusterID != rec.ClusterID {
		t.Fatalf("got %+v", got)
	}
	if len(got.MemberHashes) != 2 || got.MemberHashes[0] != "hash-a" {
		t.Fatalf("members: %v", got.MemberHashes)
	}
	if got.UpdatedAt != "2026-10-11T12:00:00Z" {
		t.Fatalf("updated_at: %q", got.UpdatedAt)
	}
}

func TestPutClusterReplacesMembers(t *testing.T) {
	st, ctx := testStore(t)
	rec := sampleCluster()
	if err := st.PutCluster(ctx, rec); err != nil {
		t.Fatal(err)
	}
	rec.Fingerprint = "fp-two"
	rec.MemberHashes = []string{"hash-c"}
	rec.Size = 1
	if err := st.PutCluster(ctx, rec); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetClusterByFingerprint(ctx, "fp-two")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.MemberHashes) != 1 || got.MemberHashes[0] != "hash-c" {
		t.Fatalf("members: %v", got.MemberHashes)
	}
}

func TestGetClusterByFingerprintNotFound(t *testing.T) {
	st, ctx := testStore(t)
	_, err := st.GetClusterByFingerprint(ctx, "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeNotFound {
		t.Fatalf("code: %v", err)
	}
}

func TestPutClusterRequiresDeadline(t *testing.T) {
	st, _ := testStore(t)
	err := st.PutCluster(context.Background(), sampleCluster())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPutClusterNilContext(t *testing.T) {
	st, _ := testStore(t)
	var nilCtx context.Context //nolint:staticcheck // intentional nil context guard test
	err := st.PutCluster(nilCtx, sampleCluster())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPutClusterRejectsEmptyMember(t *testing.T) {
	st, ctx := testStore(t)
	rec := sampleCluster()
	rec.MemberHashes = []string{"hash-a", ""}
	rec.Size = 2
	err := st.PutCluster(ctx, rec)
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func TestPutClusterRejectsSizeMismatch(t *testing.T) {
	st, ctx := testStore(t)
	rec := sampleCluster()
	rec.Size = 3
	err := st.PutCluster(ctx, rec)
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}
