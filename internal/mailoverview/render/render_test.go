package render

import (
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailoverview"
)

func TestRenderHTML_sections_and_no_external_refs(t *testing.T) {
	m := mailoverview.Model{
		SchemaVersion: mailoverview.SchemaVersion,
		GeneratedAt:   "2025-01-01T00:00:00Z",
		LedgerPath:    "/x/ledger.db",
		MailStorePath: "/y/pimdir",
		CLIVersion:    "test",
		Provenance:    mailoverview.Provenance{CompleteRows: 1, MailMatched: 1},
		Months: []mailoverview.MonthSection{
			{
				Key:   "2025-10",
				Label: "Oct 2025",
				Summary: mailoverview.SummaryStats{
					Classified: 1, JevSkips: 0, EssenceChats: 1, EmbedsSaved: 0,
					Sparkline: []int{1, 2}, SparkMonths: []string{"2025-09", "2025-10"},
				},
				Attention: mailoverview.AttentionBlock{
					UrgencyHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}, Threshold: 0.5},
					Donut:       []mailoverview.DonutSlice{{Label: "Quiet", Count: 1}},
				},
				Themes:  []mailoverview.ThemeCount{{Slot: "Other", Count: 1}},
				Heatmap: mailoverview.HeatmapData{WeekLabels: []string{"w"}, ThemeLabels: []string{"Other"}, Cells: [][]int{{1}}},
				KindTree: mailoverview.KindTreeBlock{
					Trunk: []mailoverview.KindTrunkNode{{FirstSlot: "root", Count: 1}},
					Paths: []mailoverview.KindPathRow{{Path: "root > leaf", Count: 1, Hashes: []string{"leafh"}}},
				},
				Fluff: mailoverview.FluffBlock{ByWeek: []mailoverview.WeekFluff{{WeekKey: "w", Label: "w", Minutes: 1}}},
				Pipeline: mailoverview.PipelineBlock{
					HopFlow: mailoverview.HopFlow{
						Edges: []mailoverview.HopEdge{
							{From: "Messages", To: "Jev_skip", Count: 0},
							{From: "Messages", To: "Essence", Count: 1},
							{From: "Jev_skip", To: "Classified", Count: 0},
							{From: "Essence", To: "Classified", Count: 1},
						},
					},
				},
				Trust: mailoverview.TrustBlock{
					CosineHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
					JudgeHist:  mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
				},
				Messages: []mailoverview.MessageRow{{MessageLine: mailoverview.MessageLine{ObjectHash: "h", Date: "d"}}},
			},
		},
		All: mailoverview.MonthSection{
			Key: "all", Label: "All",
			Summary: mailoverview.SummaryStats{Classified: 1, Sparkline: []int{1, 2}, SparkMonths: []string{"a", "b"}},
			Attention: mailoverview.AttentionBlock{
				UrgencyHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
				Donut:       []mailoverview.DonutSlice{{Label: "Quiet", Count: 1}},
			},
			Themes:  []mailoverview.ThemeCount{{Slot: "Other", Count: 1}},
			Heatmap: mailoverview.HeatmapData{WeekLabels: []string{"w"}, ThemeLabels: []string{"Other"}, Cells: [][]int{{1}}},
			KindTree: mailoverview.KindTreeBlock{
				Outliers: []mailoverview.KindOutlierLeaf{{FirstSlot: "x", Kind: "x > y", Why: "only"}},
				Paths:    []mailoverview.KindPathRow{{Path: "x > y", Count: 1, Why: "w"}},
			},
			Fluff: mailoverview.FluffBlock{
				ByWeek:     []mailoverview.WeekFluff{{WeekKey: "w", Minutes: 0.1}},
				TopSenders: []mailoverview.SenderFluff{{Sender: "a@x", Minutes: 0.1, Messages: 2}},
			},
			Pipeline: mailoverview.PipelineBlock{
				HopFlow:      mailoverview.HopFlow{Edges: []mailoverview.HopEdge{{From: "Messages", To: "Essence", Count: 1}, {From: "Essence", To: "Classified", Count: 1}}},
				SkipRateWeek: []mailoverview.WeekPoint{{WeekKey: "a", Rate: 0.5}, {WeekKey: "b", Rate: 0.6}},
				PromptHashes: map[string]int{"h1": 1},
			},
			Trust: mailoverview.TrustBlock{
				CosineHist:  mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
				JudgeHist:   mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
				ErrorGroups: []mailoverview.ErrorGroup{{Error: "e", Rows: 1, TotalAttempts: 1}},
			},
			Messages: []mailoverview.MessageRow{{MessageLine: mailoverview.MessageLine{ObjectHash: "h"}}},
		},
		Shapes: []mailoverview.ShapeInfo{{Fingerprint: "f", Status: hoststore.ShapeStatusActive}},
	}
	html, err := RenderHTML(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"sec-summary-all", "sec-attention-all", "sec-themes-all", "sec-fluff-all", "sec-pipeline-all", "sec-trust-all", "sec-messages-all"} {
		if !strings.Contains(html, `id="`+id+`"`) {
			t.Fatalf("missing %s", id)
		}
	}
	if !strings.Contains(html, "Kind tree") || !strings.Contains(html, `class="kbadge" title="w"`) {
		t.Fatal("missing kind tree with one-off badge")
	}
	if !strings.Contains(html, `data-hashes="leafh"`) {
		t.Fatal("kind tree rows should carry message hashes")
	}
	lower := strings.ToLower(html)
	if strings.Contains(lower, `src="http`) || strings.Contains(lower, `href="http`) {
		t.Fatal("external http ref")
	}
	if strings.Contains(html, "ZgotmplZ") {
		t.Fatal("template escaped style/script; expected raw CSS and JS")
	}
	if !strings.Contains(html, ".summary-tiles") {
		t.Fatal("inlined CSS missing summary-tiles rule")
	}
	if !strings.Contains(html, `data-month="2025-10" aria-selected="true"`) {
		t.Fatal("expected latest calendar month tab selected")
	}
	needIdx := strings.Index(html, "What needs you")
	jevIdx := strings.Index(html, "Jev skips")
	if needIdx < 0 || jevIdx < 0 || needIdx > jevIdx {
		t.Fatal("What needs you should precede Jev skips")
	}
	headerEnd := strings.Index(html, "</header>")
	ledgerIdx := strings.Index(html, "Ledger")
	if headerEnd < 0 || ledgerIdx < 0 || ledgerIdx < headerEnd {
		t.Fatal("ledger path should sit after the header")
	}
	if !strings.Contains(html, `<details class="audit">`) {
		t.Fatal("missing collapsed audit details")
	}
	if !strings.Contains(html, `id="highlight-toggle"`) || !strings.Contains(html, `id="intent-panel"`) {
		t.Fatal("missing highlight experiment controls")
	}
	if !strings.Contains(html, `id="intent-mark"`) || !strings.Contains(html, `id="delete-cart"`) {
		t.Fatal("missing deletion cart controls")
	}
	if !strings.Contains(html, "--cart-width: 30vw") || !strings.Contains(html, "cart-folder") {
		t.Fatal("missing cart sidebar folders")
	}
	if !strings.Contains(html, `id="cart-resize"`) {
		t.Fatal("missing cart resize handle")
	}
	if !strings.Contains(html, `#delete-cart:not([hidden])`) || !strings.Contains(html, `font-size: 0.85rem`) {
		t.Fatal("missing compact cart type")
	}
	if !strings.Contains(html, `#delete-cart h2`) || !strings.Contains(html, `font-size: 0.92rem`) {
		t.Fatal("missing cart heading size")
	}
	if !strings.Contains(html, `content: "Show "`) {
		t.Fatal("missing cart folder expand hint")
	}
	if !strings.Contains(html, "Start deletion job") || !strings.Contains(html, "trash-at-source") {
		t.Fatal("missing cart deletion job controls")
	}
	if !strings.Contains(html, "<th>Messages</th>") || !strings.Contains(html, "<td>2</td>") {
		t.Fatal("missing sender message counts")
	}
	if !strings.Contains(html, "--highlight-fill:") {
		t.Fatal("missing transparent highlight fill token")
	}
}

func TestHTMLOfflineRowCap(t *testing.T) {
	var msgs []mailoverview.MessageRow
	for i := 0; i < 2000; i++ {
		msgs = append(msgs, mailoverview.MessageRow{
			MessageLine: mailoverview.MessageLine{ObjectHash: strings.Repeat("a", 16), Subject: strings.Repeat("s", 200)},
		})
	}
	m := mailoverview.Model{
		SchemaVersion: 1,
		All: mailoverview.MonthSection{
			Key: "all", Label: "All",
			Summary: mailoverview.SummaryStats{Classified: 2000, Sparkline: []int{1, 2}, SparkMonths: []string{"a", "b"}},
			Attention: mailoverview.AttentionBlock{
				UrgencyHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{0}},
				Donut:       []mailoverview.DonutSlice{{Label: "Q", Count: 1}},
			},
			Themes: []mailoverview.ThemeCount{{Slot: "Other", Count: 1}},
			Heatmap: mailoverview.HeatmapData{
				WeekLabels:  []string{"w"},
				ThemeLabels: []string{"Other"},
				Cells:       [][]int{{1}},
			},
			KindTree: mailoverview.KindTreeBlock{},
			Fluff:    mailoverview.FluffBlock{},
			Pipeline: mailoverview.PipelineBlock{HopFlow: mailoverview.HopFlow{}},
			Trust:    mailoverview.TrustBlock{CosineHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{0}}, JudgeHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{0}}},
			Messages: msgs,
		},
	}
	html, err := RenderHTML(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(html) > 5_000_000 {
		t.Fatalf("html too large: %d", len(html))
	}
}
