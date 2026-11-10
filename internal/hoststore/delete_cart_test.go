package hoststore

import (
	"context"
	"testing"
)

func TestDeleteCartAddListRemove(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.AddDeleteCart(ctx, []DeleteCartItem{
		{ObjectHash: " h1 ", Sender: "Ann", Subject: "Meet", MailedAt: "2026-10-08", TLDR: "A meeting follow-up."},
		{ObjectHash: "h2", Sender: "Bob", Subject: "Ping"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ObjectHash != "h1" || got[0].Sender != "Ann" || got[1].ObjectHash != "h2" {
		t.Fatalf("list %+v", got)
	}
	if got[0].GroupID == "" || got[0].GroupID != got[1].GroupID {
		t.Fatalf("one add should share a group: %+v", got)
	}
	if err := st.AddDeleteCart(ctx, []DeleteCartItem{
		{ObjectHash: "h1", Sender: "Ann", Subject: "Meet again", TLDR: "Still a meeting."},
	}); err != nil {
		t.Fatal(err)
	}
	got, err = st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Subject != "Meet again" || got[0].TLDR != "Still a meeting." {
		t.Fatalf("upsert %+v", got)
	}
	if err := st.RemoveDeleteCart(ctx, "h1"); err != nil {
		t.Fatal(err)
	}
	got, err = st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ObjectHash != "h2" {
		t.Fatalf("after remove %+v", got)
	}
}

func TestDeleteCartRejectsEmptyHashAndMissingDeadline(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.AddDeleteCart(ctx, []DeleteCartItem{{Subject: "x"}}); err == nil {
		t.Fatal("expected empty hash")
	}
	if err := st.RemoveDeleteCart(ctx, "  "); err == nil {
		t.Fatal("expected empty remove")
	}
	if _, err := st.ListDeleteCart(context.Background()); err == nil {
		t.Fatal("expected missing deadline")
	}
}

func TestDeleteCartAddEmptyIsNoop(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.AddDeleteCart(ctx, nil); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestDeleteCartRemoveGroup(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.AddDeleteCart(ctx, []DeleteCartItem{
		{ObjectHash: "a1", GroupID: "g1", GroupLabel: "Pipelines"},
		{ObjectHash: "a2", GroupID: "g1", GroupLabel: "Pipelines"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddDeleteCart(ctx, []DeleteCartItem{
		{ObjectHash: "b1", GroupID: "g2", GroupLabel: "Newsletters"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveDeleteCartGroup(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ObjectHash != "b1" || got[0].GroupID != "g2" {
		t.Fatalf("after group remove %+v", got)
	}
	if err := st.RemoveDeleteCartGroup(ctx, "  "); err == nil {
		t.Fatal("expected empty group")
	}
}
