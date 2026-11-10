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
			{ObjectHash: "ha", Date: "2026-10-08T15:05:55Z", Sender: "a@x", SenderName: "Ann", Subject: "Meet", NeedsReply: 0.81},
			{ObjectHash: "hb", Date: "2026-10-15T22:52:44Z", Sender: "b@x", Subject: "Ping", NeedsReply: 0.66},
		},
		DecisionList: []mailoverview.MessageLine{
			{ObjectHash: "hc1", Date: "2026-10-01T00:00:00Z", Sender: "c@x", Subject: "Pay now", Decision: 0.70},
			{ObjectHash: "hc2", Date: "2026-10-09T00:00:00Z", Sender: "C@x", Subject: "Pay now!", Decision: 0.90},
			{ObjectHash: "hd", Date: "2025-12-01T00:00:00Z", Sender: "d@x", Subject: "Plan", Decision: 0.80},
		},
	}, 2026)
	if len(lists) != 2 || lists[0].Title != "Reply" || lists[1].Title != "Decide" {
		t.Fatalf("lists %+v", lists)
	}
	reply := lists[0].Rows
	if reply[0].Name != "Ann" || reply[0].Address != "a@x" || reply[0].Date != "8 Oct" {
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
	if first.Count != 2 || first.Score != 0.90 || first.Subject != "Pay now!" || first.Date != "9 Oct" {
		t.Fatalf("grouped row keeps latest subject and best score: %+v", first)
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

func TestLabels(t *testing.T) {
	if got := humanLabel("workflow-failure"); got != "Workflow failure" {
		t.Fatalf("humanLabel %q", got)
	}
	if got := weekLabel("2026-09-28"); got != "28 Sep" {
		t.Fatalf("weekLabel %q", got)
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
	for _, want := range []string{"Need a reply", ">Reply <", ">Decide <", `class="tab-sep"`, `class="minor" data-month="2016-02"`, `data-month="2025-10" aria-selected="true"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(html, `id="sec-themes-2025-10"`) {
		t.Fatal("empty themes section should be omitted")
	}
}
