package mailoverview

import (
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/pimdir"
)

func TestAttentionFromItems_quietListOmitsReply(t *testing.T) {
	got := attentionFromItems([]enriched{
		{
			row:     hoststore.OverviewRow{ObjectHash: "q1", NeedsReplyNoul: 0.1, InvolvesDecisionNoul: 0.1, Urgency: 0.2},
			email:   "spam@x",
			summary: pimdir.EmailSummary{Subject: "Sale", Date: "2026-10-01T00:00:00Z"},
		},
		{
			row:   hoststore.OverviewRow{ObjectHash: "r1", NeedsReplyNoul: 0.9, Urgency: 0.2},
			email: "a@x",
		},
		{
			row:   hoststore.OverviewRow{ObjectHash: "u1", NeedsReplyNoul: 0.1, InvolvesDecisionNoul: 0.1, Urgency: 0.8},
			email: "u@x",
		},
	})
	if got.Quiet != 1 || len(got.QuietList) != 1 || got.QuietList[0].ObjectHash != "q1" {
		t.Fatalf("quiet %+v list %+v", got.Quiet, got.QuietList)
	}
	if got.NeedsReply != 1 || len(got.ReplyList) != 1 {
		t.Fatalf("reply %+v", got.ReplyList)
	}
	if got.UrgentOnly != 1 || len(got.UrgentList) != 1 || got.UrgentList[0].ObjectHash != "u1" {
		t.Fatalf("urgent %+v list %+v", got.UrgentOnly, got.UrgentList)
	}
}

func TestDefaultSinceMonth(t *testing.T) {
	t.Parallel()
	nov := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	if got := DefaultSinceMonth(nov); got != "2026-08" {
		t.Fatalf("november: got %s", got)
	}
	aug := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if got := DefaultSinceMonth(aug); got != "2026-08" {
		t.Fatalf("august: got %s", got)
	}
	jul := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	if got := DefaultSinceMonth(jul); got != "2025-08" {
		t.Fatalf("july: got %s", got)
	}
}

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
	m := BuildModel(ref, loc, Input{SinceMonth: "2025-10", UntilMonth: "2025-11"}, rows, summaries, "/ledger", "/mail", "test", "abc", shapes, errRows, nil)
	if len(m.Months) != 2 {
		t.Fatalf("months %d want 2", len(m.Months))
	}
	for _, ms := range m.Months {
		if ms.Key == "unknown" {
			t.Fatal("unknown month should be filtered when since/until set")
		}
	}
	all := m.All
	if all.Summary.Classified != 2 {
		t.Fatalf("all classified %d", all.Summary.Classified)
	}
	if all.Summary.JevSkips != 1 || all.Summary.EssenceChats != 1 {
		t.Fatalf("skips/essence %d %d", all.Summary.JevSkips, all.Summary.EssenceChats)
	}
	if all.Summary.EmbedsSaved != 3 {
		t.Fatalf("embeds %d", all.Summary.EmbedsSaved)
	}
	if len(m.All.Trust.ErrorGroups) != 1 || m.All.Trust.ErrorGroups[0].TotalAttempts != 2 {
		t.Fatalf("error groups %+v", m.All.Trust.ErrorGroups)
	}
	if m.All.Pipeline.PromptHashes["ph1"] != 1 {
		t.Fatalf("prompt hashes %+v", m.All.Pipeline.PromptHashes)
	}
	heatSum := 0
	for _, row := range all.Heatmap.Cells {
		for _, c := range row {
			heatSum += c
		}
	}
	if heatSum != all.Summary.Classified {
		t.Fatalf("heatmap %d != classified %d weeks %v", heatSum, all.Summary.Classified, all.Heatmap.WeekLabels)
	}
	for _, label := range all.Heatmap.WeekLabels {
		if label == NoWeekKey {
			t.Fatal("undated mail should leave All when since/until is set")
		}
	}
}

func TestBuildModel_actionedStaysOnHappenedKindTree(t *testing.T) {
	loc := time.UTC
	summaries := map[string]pimdir.EmailSummary{
		"h1": {Date: "2025-10-15T12:00:00Z", Subject: "A", Sender: "a@ex.com"},
		"h2": {Date: "2025-10-16T12:00:00Z", Subject: "B", Sender: "b@ex.com"},
	}
	rows := []hoststore.OverviewRow{
		{ObjectHash: "h1", TermID: "t", Kind: "alpha > pull-request", Strategy: "essence", Cosine: 0.9, JudgeScore: 0.85, Urgency: 0.2, NeedsReplyNoul: 0.1, InvolvesDecisionNoul: 0.1, AttentionScored: true},
		{ObjectHash: "h2", TermID: "t", Kind: "alpha > article", Strategy: "essence", Cosine: 0.7, JudgeScore: 0.6, Urgency: 0.6, NeedsReplyNoul: 0.55, InvolvesDecisionNoul: 0.1, AttentionScored: true},
	}
	ref := time.Date(2025, 12, 1, 0, 0, 0, 0, loc)
	m := BuildModel(ref, loc, Input{SinceMonth: "2025-10", UntilMonth: "2025-10"}, rows, summaries, "/ledger", "/mail", "test", "abc", nil, nil, map[string]maildelete.ActionedInfo{
		"h1": {Outcome: maildelete.OutcomeTrashedAtSource, Date: "2025-10-15"},
	})
	if m.All.Summary.Classified != 2 {
		t.Fatalf("classified %d want 2, report keeps actioned mail", m.All.Summary.Classified)
	}
	if kindPathTotal(m.All.KindTree) != 1 {
		t.Fatalf("pending kind tree %+v", m.All.KindTree.Paths)
	}
	if kindPathTotal(m.All.HappenedKindTree) != 2 {
		t.Fatalf("happened kind tree %+v", m.All.HappenedKindTree.Paths)
	}
	if len(m.Months) != 1 || m.Months[0].Key != "2025-10" {
		t.Fatalf("months %+v", m.Months)
	}
	if kindPathTotal(m.Months[0].KindTree) != 1 || kindPathTotal(m.Months[0].HappenedKindTree) != 2 {
		t.Fatalf("month trees pending %+v happened %+v", m.Months[0].KindTree.Paths, m.Months[0].HappenedKindTree.Paths)
	}
}

func kindPathTotal(block KindTreeBlock) int {
	n := 0
	for _, p := range block.Paths {
		n += p.Count
	}
	return n
}

func TestThemesAndHeatmap_undatedColumn(t *testing.T) {
	_, hm := themesAndHeatmap([]enriched{
		{weekKey: "2026-08-31", theme: themeOther, row: hoststore.OverviewRow{ObjectHash: "dated"}},
		{weekKey: "", theme: themeOther, row: hoststore.OverviewRow{ObjectHash: "nodate-a"}},
		{weekKey: "", theme: "engineering", row: hoststore.OverviewRow{ObjectHash: "nodate-b"}},
	})
	if len(hm.WeekLabels) != 2 || hm.WeekLabels[0] != "2026-08-31" || hm.WeekLabels[1] != NoWeekKey {
		t.Fatalf("weeks %v", hm.WeekLabels)
	}
	sum := 0
	for _, row := range hm.Cells {
		for _, c := range row {
			sum += c
		}
	}
	if sum != 3 {
		t.Fatalf("heatmap sum %d cells %+v", sum, hm.Cells)
	}
	eng := -1
	other := -1
	for i, lab := range hm.ThemeLabels {
		switch lab {
		case "engineering":
			eng = i
		case themeOther:
			other = i
		}
	}
	if eng < 0 || other < 0 {
		t.Fatalf("themes %v", hm.ThemeLabels)
	}
	if hm.Cells[1][eng] != 1 || hm.Cells[1][other] != 1 {
		t.Fatalf("no-date column %+v", hm.Cells[1])
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
