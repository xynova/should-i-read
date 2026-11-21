package mailoverview

import (
	"testing"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
)

func TestDropActionedRows(t *testing.T) {
	rows := []hoststore.OverviewRow{
		{ObjectHash: "live"},
		{ObjectHash: "trash"},
	}
	out := DropActionedRows(rows, map[string]maildelete.ActionedInfo{
		"trash": {Outcome: maildelete.OutcomeAlreadyInTrash},
	})
	if len(out) != 1 || out[0].ObjectHash != "live" {
		t.Fatalf("%+v", out)
	}
}

func TestGroupActioned(t *testing.T) {
	rows := GroupActioned(map[string]maildelete.ActionedInfo{
		"h2": {
			Sender: "notifications@github.com", Outcome: maildelete.OutcomeAlreadyInTrash,
			Date: "2026-09-30",
		},
		"h1": {
			Sender: "notifications@github.com", Outcome: maildelete.OutcomeAlreadyInTrash,
			Date: "2026-09-19",
		},
		"h3": {Sender: "gitlab@mg.gitlab.com", Outcome: maildelete.OutcomeAlreadyInTrash},
	})
	if len(rows) != 2 {
		t.Fatalf("%d", len(rows))
	}
	if rows[0].Sender != "gitlab@mg.gitlab.com" || rows[0].Count != 1 {
		t.Fatalf("%+v", rows[0])
	}
	gh := rows[1]
	if gh.Sender != "notifications@github.com" || gh.Count != 2 {
		t.Fatalf("%+v", gh)
	}
	if gh.From != "19 Sep 2026" || gh.Until != "30 Sep 2026" {
		t.Fatalf("when %+v", gh)
	}
	if gh.WhenLabel() != "19 Sep 2026 through 30 Sep 2026" {
		t.Fatalf("label %q", gh.WhenLabel())
	}
}

func TestActionedOutcomeAndSenderCounts(t *testing.T) {
	rows := []ActionedRow{
		{Sender: "a@x", Count: 2, Outcome: maildelete.OutcomeAlreadyInTrash},
		{Sender: "b@x", Count: 5, Outcome: maildelete.OutcomeTrashedAtSource},
		{Sender: "a@x", Count: 1, Outcome: maildelete.OutcomeAckedAtSource},
	}
	outcomes := ActionedOutcomeCounts(rows)
	if len(outcomes) != 3 || outcomes[0].Label != maildelete.OutcomeTrashedAtSource || outcomes[0].Count != 5 {
		t.Fatalf("outcomes %+v", outcomes)
	}
	if outcomes[2].Label != maildelete.OutcomeAckedAtSource || outcomes[2].Count != 1 {
		t.Fatalf("acked %+v", outcomes)
	}
	senders := ActionedSenderCounts(rows, 8)
	if len(senders) != 2 || senders[0].Label != "b@x" || senders[0].Count != 5 {
		t.Fatalf("senders %+v", senders)
	}
	if senders[1].Label != "a@x" || senders[1].Count != 3 {
		t.Fatalf("a@x %+v", senders[1])
	}
}

func TestFilterCartAdds(t *testing.T) {
	toAdd, inCart, actioned := filterCartAdds(
		[]hoststore.DeleteCartItem{{ObjectHash: "already"}},
		[]hoststore.DeleteCartItem{
			{ObjectHash: "already"},
			{ObjectHash: "trash"},
			{ObjectHash: "live"},
		},
		map[string]maildelete.ActionedInfo{"trash": {Outcome: maildelete.OutcomeAlreadyInTrash}},
	)
	if inCart != 1 || actioned != 1 || len(toAdd) != 1 || toAdd[0].ObjectHash != "live" {
		t.Fatalf("toAdd %+v inCart %d actioned %d", toAdd, inCart, actioned)
	}
}
