package hoststore

import (
	"context"
	"testing"
)

func TestUpsertOutcomeTrashWinsOverKeep(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.UpsertOutcome(ctx, MessageOutcome{
		ObjectHash: "hash-a",
		Outcome:    OutcomeKept,
		Source:     OutcomeSourceOverviewKeep,
		Sender:     "a@x",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertOutcome(ctx, MessageOutcome{
		ObjectHash: "hash-a",
		Outcome:    OutcomeTrashedAtSource,
		Source:     OutcomeSourceDeleteJob,
		JobID:      "job-1",
		Sender:     "a@x",
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetOutcome(ctx, "hash-a")
	if err != nil || !ok || got.Outcome != OutcomeTrashedAtSource {
		t.Fatalf("got %+v ok %v err %v", got, ok, err)
	}
	if err := st.UpsertOutcome(ctx, MessageOutcome{
		ObjectHash: "hash-a",
		Outcome:    OutcomeKept,
		Source:     OutcomeSourceOverviewKeep,
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err = st.GetOutcome(ctx, "hash-a")
	if err != nil || !ok || got.Outcome != OutcomeTrashedAtSource {
		t.Fatalf("keep must not overwrite trash: %+v", got)
	}
}

func TestClearKeptOutcomeLeavesTrash(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.UpsertOutcome(ctx, MessageOutcome{
		ObjectHash: "keep-1", Outcome: OutcomeKept, Source: OutcomeSourceOverviewKeep,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.ClearKeptOutcome(ctx, "keep-1"); err != nil {
		t.Fatal(err)
	}
	_, ok, err := st.GetOutcome(ctx, "keep-1")
	if err != nil || ok {
		t.Fatalf("kept should be gone ok=%v err=%v", ok, err)
	}
	if err := st.UpsertOutcome(ctx, MessageOutcome{
		ObjectHash: "trash-1", Outcome: OutcomeTrashedAtSource, Source: OutcomeSourceDeleteJob,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.ClearKeptOutcome(ctx, "trash-1"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetOutcome(ctx, "trash-1")
	if err != nil || !ok || got.Outcome != OutcomeTrashedAtSource {
		t.Fatalf("trash must stay %+v ok %v err %v", got, ok, err)
	}
}

func TestUpsertOutcomeRequiresDeadline(t *testing.T) {
	st, _ := testStore(t)
	err := st.UpsertOutcome(context.Background(), MessageOutcome{
		ObjectHash: "h",
		Outcome:    OutcomeKept,
		Source:     OutcomeSourceOverviewKeep,
	})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestGetOutcomeRoundTrip(t *testing.T) {
	st, ctx := testStore(t)
	want := MessageOutcome{
		ObjectHash: "hash-b",
		Outcome:    OutcomeAlreadyInTrash,
		Source:     OutcomeSourceReplicaFolder,
		Sender:     "b@y",
	}
	if err := st.UpsertOutcome(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetOutcome(ctx, "hash-b")
	if err != nil || !ok {
		t.Fatalf("err %v ok %v", err, ok)
	}
	if got.Outcome != want.Outcome || got.Source != want.Source || got.Sender != want.Sender {
		t.Fatalf("got %+v", got)
	}
	m, err := st.GetOutcomes(ctx, []string{"hash-b", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m["hash-b"].Outcome != OutcomeAlreadyInTrash {
		t.Fatalf("map %+v", m)
	}
}

func TestCountOutcomes(t *testing.T) {
	st, ctx := testStore(t)
	for _, o := range []MessageOutcome{
		{ObjectHash: "t1", Outcome: OutcomeTrashedAtSource, Source: OutcomeSourceDeleteJob},
		{ObjectHash: "t2", Outcome: OutcomeAlreadyInTrash, Source: OutcomeSourceReplicaFolder},
		{ObjectHash: "k1", Outcome: OutcomeKept, Source: OutcomeSourceOverviewKeep},
	} {
		if err := st.UpsertOutcome(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	c, err := st.CountOutcomes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c.TrashedAtSource != 1 || c.AlreadyInTrash != 1 || c.Kept != 1 {
		t.Fatalf("counts %+v", c)
	}
	list, err := st.ListOutcomes(ctx)
	if err != nil || len(list) != 3 {
		t.Fatalf("list len %d err %v", len(list), err)
	}
}

func TestUpsertOutcomeAckedHidesAndKeepCannotOverwrite(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.UpsertOutcome(ctx, MessageOutcome{
		ObjectHash: "h1", Outcome: OutcomeKept, Source: OutcomeSourceOverviewKeep, Sender: "a@x",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertOutcome(ctx, MessageOutcome{
		ObjectHash: "h1", Outcome: OutcomeAcked, Source: OutcomeSourceAckJob, JobID: "ack-1", Sender: "a@x",
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetOutcome(ctx, "h1")
	if err != nil || !ok || got.Outcome != OutcomeAcked {
		t.Fatalf("acked should win over keep %+v ok %v err %v", got, ok, err)
	}
	if err := st.UpsertOutcome(ctx, MessageOutcome{
		ObjectHash: "h1", Outcome: OutcomeKept, Source: OutcomeSourceOverviewKeep,
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err = st.GetOutcome(ctx, "h1")
	if err != nil || !ok || got.Outcome != OutcomeAcked {
		t.Fatalf("keep must not overwrite acked %+v", got)
	}
}
