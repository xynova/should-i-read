package mailoverview

import (
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/pimdir"
)

func TestBuildModel_twoMonths_unknown_filtered(t *testing.T) {
	loc := time.Local
	summaries := map[string]pimdir.EmailSummary{
		"h1": {Date: "2025-10-15T12:00:00Z", Subject: "A", Sender: "a@ex.com"},
		"h2": {Date: "2025-11-10T12:00:00Z", Subject: "B", Sender: "b@ex.com"},
		"h3": {Subject: "C", Sender: "c@ex.com"},
	}
	rows := []hoststore.OverviewRow{
		{ObjectHash: "h1", TermID: "t", Kind: "alpha > pull-request", Strategy: mailreport.StrategyShapeConfirm, Cosine: 0.9, JudgeScore: 0.85, Urgency: 0.2, NeedsReplyNoul: 0.1, InvolvesDecisionNoul: 0.1, AttentionScored: true},
		{ObjectHash: "h2", TermID: "t", Kind: "alpha > article", Strategy: "essence", Cosine: 0.7, JudgeScore: 0.6, Urgency: 0.6, NeedsReplyNoul: 0.55, InvolvesDecisionNoul: 0.1, AttentionScored: true},
		{ObjectHash: "h3", TermID: "t", Kind: "beta > invitation", Strategy: "essence", Cosine: 0.4, JudgeScore: 0.5, Urgency: 0.3, AttentionScored: false},
	}
	shapes := []hoststore.MailShapeRow{
		{Fingerprint: "fp1", Status: hoststore.ShapeStatusActive, PromptHash: "ph1", HitCount: 1},
	}
	errRows := []hoststore.ErrorRow{
		{ObjectHash: "e1", Error: "fail", AttemptCount: 2, UpdatedAt: "2025-01-01T00:00:00Z"},
	}
	ref := time.Date(2025, 12, 1, 0, 0, 0, 0, loc)
	m := BuildModel(ref, loc, Input{SinceMonth: "2025-10", UntilMonth: "2025-11"}, rows, summaries, "/ledger", "/mail", "test", "abc", shapes, errRows)
	if len(m.Months) != 2 {
		t.Fatalf("months %d want 2", len(m.Months))
	}
	for _, ms := range m.Months {
		if ms.Key == "unknown" {
			t.Fatal("unknown month should be filtered when since/until set")
		}
	}
	all := m.All
	if all.Summary.Classified != 3 {
		t.Fatalf("all classified %d", all.Summary.Classified)
	}
	if all.Summary.JevSkips != 1 || all.Summary.EssenceChats != 2 {
		t.Fatalf("skips/essence %d %d", all.Summary.JevSkips, all.Summary.EssenceChats)
	}
	if all.Summary.EmbedsSaved != 3 {
		t.Fatalf("embeds %d", all.Summary.EmbedsSaved)
	}
	if len(all.KindTree.Outliers) == 0 {
		t.Fatal("expected outlier first slots")
	}
	found := false
	for _, o := range all.KindTree.Outliers {
		if o.FirstSlot == "beta" {
			found = true
			if o.Why == "" {
				t.Fatal("empty why")
			}
		}
	}
	if !found {
		t.Fatal("missing beta outlier")
	}
	if len(m.All.Trust.ErrorGroups) != 1 || m.All.Trust.ErrorGroups[0].TotalAttempts != 2 {
		t.Fatalf("error groups %+v", m.All.Trust.ErrorGroups)
	}
	if m.All.Pipeline.PromptHashes["ph1"] != 1 {
		t.Fatalf("prompt hashes %+v", m.All.Pipeline.PromptHashes)
	}
}

func TestKindTree(t *testing.T) {
	items := []enriched{
		{row: hoststore.OverviewRow{Kind: "shared > a", ObjectHash: "ha"}, month: "2025-10"},
		{row: hoststore.OverviewRow{Kind: "shared > b", ObjectHash: "hb"}, month: "2025-10"},
		{row: hoststore.OverviewRow{Kind: "onlyone > z", ObjectHash: "hz"}, month: "2025-10"},
	}
	kt := kindTreeFromItems(items, "2025-10")
	if len(kt.Trunk) != 1 || kt.Trunk[0].FirstSlot != "shared" {
		t.Fatalf("trunk %+v", kt.Trunk)
	}
	if len(kt.Outliers) != 1 || kt.Outliers[0].FirstSlot != "onlyone" {
		t.Fatalf("outliers %+v", kt.Outliers)
	}
	found := false
	for _, p := range kt.Paths {
		if p.Path != "shared > a" {
			continue
		}
		found = true
		if len(p.Hashes) != 1 || p.Hashes[0] != "ha" {
			t.Fatalf("path hashes %+v", p.Hashes)
		}
	}
	if !found {
		t.Fatal("missing shared > a path hashes")
	}
}

func TestFluffSenderMessageCount(t *testing.T) {
	got := fluffFromItems([]enriched{
		{email: "a@x", row: hoststore.OverviewRow{ObjectHash: "h1"}},
		{email: "a@x", row: hoststore.OverviewRow{ObjectHash: "h2"}},
		{email: "b@x", row: hoststore.OverviewRow{ObjectHash: "h3"}},
	})
	if len(got.TopSenders) != 2 {
		t.Fatalf("senders %+v", got.TopSenders)
	}
	if got.TopSenders[0].Sender != "a@x" || got.TopSenders[0].Messages != 2 {
		t.Fatalf("top %+v", got.TopSenders[0])
	}
	if got.TopSenders[1].Sender != "b@x" || got.TopSenders[1].Messages != 1 {
		t.Fatalf("second %+v", got.TopSenders[1])
	}
}
