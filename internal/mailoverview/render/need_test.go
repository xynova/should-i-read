package render

import (
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/mailoverview"
)

func TestAttentionTiles(t *testing.T) {
	tiles := attentionTiles(mailoverview.AttentionBlock{
		NeedsReply: 9, Decision: 27, UrgentOnly: 34, Quiet: 815,
	})
	if len(tiles) != 4 {
		t.Fatalf("len %d", len(tiles))
	}
	if tiles[0].Tone != "reply" || tiles[0].Count != 9 || tiles[3].Count != 815 {
		t.Fatalf("tiles %+v", tiles)
	}
}

func TestActionLists_splitGroupAndSort(t *testing.T) {
	lists := actionLists(mailoverview.AttentionBlock{
		ReplyList: []mailoverview.MessageLine{
			{ObjectHash: "ha", Date: "2026-10-08T15:05:55Z", Sender: "a@x", SenderName: "Ann", Subject: "Meet", About: "Book a time.", NeedsReply: 0.81},
			{ObjectHash: "hb", Date: "2026-10-15T22:52:44Z", Sender: "b@x", Subject: "Ping", NeedsReply: 0.66},
		},
		DecisionList: []mailoverview.MessageLine{
			{ObjectHash: "hc1", Date: "2026-10-01T00:00:00Z", Sender: "c@x", Subject: "Pay now", Decision: 0.70},
			{ObjectHash: "hc2", Date: "2026-10-09T00:00:00Z", Sender: "C@x", Subject: "Pay now!", About: "Invoice due.", Decision: 0.90},
			{ObjectHash: "hd", Date: "2025-12-01T00:00:00Z", Sender: "d@x", Subject: "Plan", Decision: 0.80},
		},
		UrgentList: []mailoverview.MessageLine{
			{ObjectHash: "hu", Date: "2026-10-20T00:00:00Z", Sender: "u@x", SenderName: "Urgent Co", Subject: "Deadline", About: "Due today.", Urgency: 0.91},
		},
	}, 2026)
	if len(lists) != 3 || lists[0].Title != "Reply" || lists[1].Title != "Decide" || lists[2].Title != "Urgent" {
		t.Fatalf("lists %+v", lists)
	}
	if lists[2].Tone != "urgent" || lists[2].Rows[0].Name != "Urgent Co" || lists[2].Rows[0].TLDR != "Due today." {
		t.Fatalf("urgent row %+v", lists[2])
	}
	reply := lists[0].Rows
	if reply[0].Name != "Ann" || reply[0].Address != "a@x" || reply[0].Date != "8 Oct" || reply[0].TLDR != "Book a time." {
		t.Fatalf("reply row %+v", reply[0])
	}
	if reply[1].Name != "b@x" || reply[1].Address != "" {
		t.Fatalf("name falls back to address: %+v", reply[1])
	}
	decide := lists[1]
	if decide.Messages != 3 || len(decide.Rows) != 2 {
		t.Fatalf("expected repeats grouped: %+v", decide)
	}
	first := decide.Rows[0]
	if first.Count != 2 || first.Score != 0.90 || first.Subject != "Pay now!" || first.Date != "9 Oct" || first.TLDR != "Invoice due." {
		t.Fatalf("grouped row keeps latest subject, TLDR, and best score: %+v", first)
	}
	if first.HashAttr != "hc1,hc2" {
		t.Fatalf("grouped hashes %q", first.HashAttr)
	}
	if decide.Rows[1].Date != "1 Dec 2025" {
		t.Fatalf("other year keeps the year: %+v", decide.Rows[1])
	}
}

func TestActionLists_extraRowsHidden(t *testing.T) {
	var lines []mailoverview.MessageLine
	for i := 0; i < visibleActionRows+3; i++ {
		lines = append(lines, mailoverview.MessageLine{Sender: "s@x", Subject: strings.Repeat("x", i+1), Decision: 0.6})
	}
	lists := actionLists(mailoverview.AttentionBlock{DecisionList: lines}, 2026)
	if lists[0].ExtraCount != 3 || !lists[0].Rows[visibleActionRows].Extra || lists[0].Rows[0].Extra {
		t.Fatalf("extra rows %+v", lists[0])
	}
}

func TestGroupDeleteSenders_bySenderExpandAndExtra(t *testing.T) {
	lines := []mailoverview.MessageLine{
		{ObjectHash: "h1", Sender: "spam@x", SenderName: "Spam", Subject: "One", Date: "2026-10-02T00:00:00Z"},
		{ObjectHash: "h2", Sender: "spam@x", SenderName: "Spam", Subject: "Two", Date: "2026-10-09T00:00:00Z"},
		{ObjectHash: "h3", Sender: "other@x", Subject: "Hi", Date: "2026-10-01T00:00:00Z"},
	}
	got := groupDeleteSenders(lines, 2026, "all")
	if got.Title != "Delete" || got.Messages != 3 || len(got.Senders) != 2 {
		t.Fatalf("got %+v", got)
	}
	top := got.Senders[0]
	if top.Name != "Spam" || top.Address != "spam@x" || top.Messages != 2 || top.HashAttr != "h1,h2" {
		t.Fatalf("top %+v", top)
	}
	if top.Minutes < 0.16 || top.Minutes > 0.17 {
		t.Fatalf("minutes %v", top.Minutes)
	}
	if top.ExpandID != "del-all-0" || len(top.Entries) != 2 || top.Entries[0].Subject != "Two" {
		t.Fatalf("entries %+v", top)
	}
	var many []mailoverview.MessageLine
	for i := 0; i < visibleActionRows+2; i++ {
		many = append(many, mailoverview.MessageLine{Sender: strings.Repeat("a", i+1) + "@x", ObjectHash: "h", Subject: "s"})
	}
	extra := groupDeleteSenders(many, 2026, "2026-10")
	if extra.ExtraCount != 2 || !extra.Senders[visibleActionRows].Extra || extra.Senders[0].Extra {
		t.Fatalf("extra %+v", extra.ExtraCount)
	}
}

func TestGroupDeleteSenders_splitsSharedNotificationAddress(t *testing.T) {
	got := groupDeleteSenders([]mailoverview.MessageLine{
		{ObjectHash: "s1", Sender: "notifications@github.com", SenderName: "sonarqubecloud[bot]", Subject: "Analyze", Date: "2026-11-07T00:00:00Z"},
		{ObjectHash: "g1", Sender: "notifications@github.com", SenderName: "GitHub", Subject: "PR run failed", Date: "2026-11-07T01:00:00Z"},
		{ObjectHash: "g2", Sender: "notifications@github.com", SenderName: "GitHub", Subject: "Run failed", Date: "2026-11-06T00:00:00Z"},
	}, 2026, "nov")
	if len(got.Senders) != 2 {
		t.Fatalf("senders %+v", got.Senders)
	}
	var sonar, gh DeleteSenderRow
	for _, s := range got.Senders {
		switch s.Name {
		case "sonarqubecloud[bot]":
			sonar = s
		case "GitHub":
			gh = s
		}
	}
	if sonar.Messages != 1 || sonar.HashAttr != "s1" || sonar.Address != "notifications@github.com" {
		t.Fatalf("sonar %+v", sonar)
	}
	if gh.Messages != 2 || gh.HashAttr != "g1,g2" {
		t.Fatalf("github %+v", gh)
	}
}

func TestLabels(t *testing.T) {
	if got := humanLabel("workflow-failure"); got != "Workflow failure" {
		t.Fatalf("humanLabel %q", got)
	}
	if got := weekLabel("2026-09-28"); got != "28 Sep" {
		t.Fatalf("weekLabel %q", got)
	}
	if got := weekLabel(mailoverview.NoWeekKey); got != "No date" {
		t.Fatalf("weekLabel no-date %q", got)
	}
	if got := generatedLabel("2026-11-06T22:50:18Z"); got != "6 Nov 2026, 22:50 UTC" {
		t.Fatalf("generatedLabel %q", got)
	}
}

func TestRenderHTML_tilesListsAndMinorTabs(t *testing.T) {
	att := mailoverview.AttentionBlock{
		NeedsReply: 1, Decision: 1,
		ReplyList: []mailoverview.MessageLine{
			{Date: "2025-10-02T00:00:00Z", Sender: "a@x", Subject: "Hi", NeedsReply: 0.9},
		},
		DecisionList: []mailoverview.MessageLine{
			{Date: "2025-10-03T00:00:00Z", Sender: "b@x", Subject: "Go?", Decision: 0.8},
		},
		UrgentList: []mailoverview.MessageLine{
			{Date: "2025-10-05T00:00:00Z", Sender: "u@x", Subject: "Due", About: "Time-sensitive.", Urgency: 0.9},
		},
		QuietList: []mailoverview.MessageLine{
			{ObjectHash: "hq", Date: "2025-10-04T00:00:00Z", Sender: "crap@x", Subject: "Sale"},
		},
		UrgencyHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
	}
	trust := mailoverview.TrustBlock{
		CosineHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{0}},
		JudgeHist:  mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{0}},
	}
	m := mailoverview.Model{
		SchemaVersion: 1,
		GeneratedAt:   "2025-11-01T00:00:00Z",
		All: mailoverview.MonthSection{
			Key: "all", Label: "All",
			Summary:   mailoverview.SummaryStats{Classified: 21},
			Attention: att, Trust: trust,
		},
		Months: []mailoverview.MonthSection{
			{Key: "2016-02", Label: "Feb 2016", Summary: mailoverview.SummaryStats{Classified: 1}, Trust: trust},
			{Key: "2025-10", Label: "Oct 2025", Summary: mailoverview.SummaryStats{Classified: 20}, Attention: att, Trust: trust},
		},
	}
	html, err := RenderHTML(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Need a reply", ">Reply <", ">Decide <", ">Urgent <", ">Delete <", `<th class="col-subject">Subject</th>`, `<th class="col-tldr">TLDR</th>`, `<th class="col-ack">Acked</th>`, "ack-mark", "seen-at-source", `id="ack-cart"`, "table-layout: fixed", `class="delete-parent"`, `class="delete-entry"`, `table.delete-table th.col-ack`, `<th class="col-mark">Mark as deletable</th>`, "delete-mark", "function onDeleteListClick", "function stageAckParent", "function parentHasFreeHashes", "function syncExclusiveRow", `class="month-more"`, `class="minor" data-month="2016-02"`, `data-month="2025-10" aria-selected="true"`, `data-page="actioned"`, `data-page="report"`, `page-actioned`, `id="sec-actioned-report"`, `id="sec-provenance"`, "function activatePage", `<h1 id="board-title">should-i-read</h1>`, `class="brand-mark"`, `class="month-row"`, `class="month-meta"`, `id="mail-sync"`, `id="mail-sync-stripe"`, "function startMailSync", "/sync", "then Sync pushes Gmail", `id="apply-source"`, `class="apply-source-icon"`, `class="apply-source-spinner"`, `aria-label="Apply at source"`, "function applyAtSource", `EventSource("/events")`, "is-progress", "function showMailSyncProgress", "reloadBoardIfSyncDone", "location.reload"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(html, "function filterAckedItems") {
		t.Fatal("Mark as deletable must not strip acked hashes")
	}
	if strings.Contains(html, `id="sec-themes-2025-10"`) {
		t.Fatal("empty themes section should be omitted")
	}
	if strings.Contains(html, "Stray dates") || strings.Contains(html, `class="tab-sep"`) {
		t.Fatal("previous months must sit in the pulldown, not a Stray dates row")
	}
}

func TestIsMinorTab(t *testing.T) {
	gen := "2026-11-15T03:52:39Z"
	cases := []struct {
		key  string
		want bool
	}{
		{key: "unknown", want: true},
		{key: "2016-02", want: true},
		{key: "2025-06", want: true},
		{key: "2026-06", want: true},
		{key: "2026-08", want: true},
		{key: "2026-09", want: true},
		{key: "2026-10", want: false},
		{key: "2026-11", want: false},
	}
	for _, tc := range cases {
		if got := isMinorTab(tc.key, gen); got != tc.want {
			t.Fatalf("%s: minor %v want %v", tc.key, got, tc.want)
		}
	}
}

func TestRenderHTML_recentLowCountOnMainRow(t *testing.T) {
	trust := mailoverview.TrustBlock{
		CosineHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{0}},
		JudgeHist:  mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{0}},
	}
	m := mailoverview.Model{
		SchemaVersion: 1,
		GeneratedAt:   "2026-11-15T03:52:39Z",
		All: mailoverview.MonthSection{
			Key: "all", Label: "All",
			Summary: mailoverview.SummaryStats{Classified: 14},
			Trust:   trust,
		},
		Months: []mailoverview.MonthSection{
			{Key: "2016-02", Label: "Feb 2016", Summary: mailoverview.SummaryStats{Classified: 1}, Trust: trust},
			{Key: "2025-06", Label: "Jun 2025", Summary: mailoverview.SummaryStats{Classified: 40}, Trust: trust},
			{Key: "2026-08", Label: "Aug 2026", Summary: mailoverview.SummaryStats{Classified: 12}, Trust: trust},
			{Key: "2026-09", Label: "Sep 2026", Summary: mailoverview.SummaryStats{Classified: 4}, Trust: trust},
			{Key: "2026-10", Label: "Oct 2026", Summary: mailoverview.SummaryStats{Classified: 8}, Trust: trust},
			{Key: "2026-11", Label: "Nov 2026", Summary: mailoverview.SummaryStats{Classified: 9}, Trust: trust},
			{Key: "unknown", Label: "Unknown month", Summary: mailoverview.SummaryStats{Classified: 3}, Trust: trust},
		},
	}
	html, err := RenderHTML(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, `class="minor" data-month="2026-10"`) || strings.Contains(html, `class="minor" data-month="2026-11"`) {
		t.Fatal("this month and last month must stay on the main row")
	}
	if !strings.Contains(html, `class="minor" data-month="2026-09"`) {
		t.Fatal("September must sit in the Previous pulldown")
	}
	if !strings.Contains(html, `data-month="2026-11" aria-selected="true"`) {
		t.Fatal("latest recent month should be selected")
	}
	if !strings.Contains(html, `class="month-more"`) || !strings.Contains(html, `aria-label="Previous months"`) {
		t.Fatal("previous months must sit in a pulldown")
	}
	if strings.Contains(html, "Stray dates") {
		t.Fatal("Stray dates separator must be gone")
	}
	tabsStart := strings.Index(html, `class="month-tabs"`)
	if tabsStart < 0 {
		t.Fatal("missing month tabs")
	}
	tabs := html[tabsStart:]
	allIdx := strings.Index(tabs, `data-month="all"`)
	moreIdx := strings.Index(tabs, `class="month-more"`)
	novIdx := strings.Index(tabs, `data-month="2026-11"`)
	if allIdx < 0 || moreIdx < 0 || novIdx < 0 {
		t.Fatal("All, Previous, and Nov 2026 must all render")
	}
	if !(allIdx < moreIdx && moreIdx < novIdx) {
		t.Fatal("earlier months must sit left of this and last month: All, then …, then Oct and Nov")
	}
	for _, want := range []string{
		`class="minor" data-month="2016-02"`,
		`class="minor" data-month="2025-06"`,
		`class="minor" data-month="2026-08"`,
		`class="minor" data-month="2026-09"`,
		`class="minor" data-month="unknown"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing previous-month tab %q", want)
		}
	}
}
