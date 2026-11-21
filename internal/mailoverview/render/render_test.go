package render

import (
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailoverview"
)

func sampleBoardModel() mailoverview.Model {
	return mailoverview.Model{
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
				TopSenders: []mailoverview.SenderFluff{{Sender: "a@x", Minutes: 0.1, Messages: 2, Hashes: []string{"h1", "h2"}}},
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
		Actioned: []mailoverview.ActionedRow{
			{Sender: "n@github.com", Count: 2, Outcome: "already in Trash", Hashes: []string{"t1", "t2"}},
		},
		Shapes: []mailoverview.ShapeInfo{{Fingerprint: "f", Status: hoststore.ShapeStatusActive}},
	}
}

func mustRenderHTML(t *testing.T, m mailoverview.Model) string {
	t.Helper()
	html, err := RenderHTML(m)
	if err != nil {
		t.Fatal(err)
	}
	return html
}

func TestRenderHTML_sections_kind_and_fluff(t *testing.T) {
	html := mustRenderHTML(t, sampleBoardModel())
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
	if !strings.Contains(html, "function kindRowOf") || !strings.Contains(html, `closest(".kind-tree")`) {
		t.Fatal("kind tree highlight must snap to one krow, not the wrapping branch")
	}
	if !strings.Contains(html, "function itemsFromKindRow") || !strings.Contains(html, `closest(".kleaf")`) {
		t.Fatal("kind tree parent rows must not stage descendant hashes")
	}
	if !strings.Contains(html, "function onKindTreeClick") || !strings.Contains(html, `closest(".kind-tree")`) {
		t.Fatal("kind tree must pick one leaf on click, not a highlight drag")
	}
	if !strings.Contains(html, `class="data fluff-senders"`) || !strings.Contains(html, `data-hashes="h1,h2"`) {
		t.Fatal("senders costing minutes must carry hashes for the deletion cart")
	}
	if !strings.Contains(html, "function onFluffSenderClick") || !strings.Contains(html, `table.fluff-senders tbody tr[data-hashes]`) {
		t.Fatal("senders costing minutes must open Mark as deletable")
	}
	if !strings.Contains(html, "function hideEmptyFluffSenders") {
		t.Fatal("costing-minutes table must hide when every sender row is already actioned")
	}
	if strings.Contains(html, "Triage time by week") || strings.Contains(html, "Deleting versus acting") {
		t.Fatal("formula triage charts must not render")
	}
	if !strings.Contains(html, "function onDeleteListClick") || !strings.Contains(html, "function toggleDeleteEntries") {
		t.Fatal("Delete list must expand entries and open Mark as deletable")
	}
	if strings.Contains(html, "function openDeleteSender") || strings.Contains(html, `askIntent(readableText(parent)`) {
		t.Fatal("Delete list sender must not open the intent panel")
	}
	if !strings.Contains(html, "function stageDeleteParent") || !strings.Contains(html, "function postCart") {
		t.Fatal("Delete list sender must Mark on the row")
	}
	if !strings.Contains(html, "var intentMemo") || !strings.Contains(html, "function forgetIntentMemo") {
		t.Fatal("Kind and fluff intent must memoize the Polypus TLDR")
	}
	if !strings.Contains(html, "forgetIntentMemo()") {
		t.Fatal("Mark as deletable must drop the intent memo")
	}
	if !strings.Contains(html, `li.is-staged > .kleaf .kname`) {
		t.Fatal("staged paint must not mark every kname under Notification")
	}
	if !strings.Contains(html, "function kindTreeBranchLi") {
		t.Fatal("parent kind lis must not look staged when one child is in the cart")
	}
	if !strings.Contains(html, "function readableText") || !strings.Contains(html, "function svgTitleLabel") {
		t.Fatal("highlight must read SVG tooltip labels so cell 7 is not sent as 77")
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
}

func TestRenderHTML_header_pages_and_report(t *testing.T) {
	html := mustRenderHTML(t, sampleBoardModel())
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
	if strings.Contains(html, `<details class="audit">`) {
		t.Fatal("pipeline and template health must sit on Report, not collapsed details")
	}
	if !strings.Contains(html, `data-page="overview"`) || !strings.Contains(html, `data-page="actioned"`) || !strings.Contains(html, `data-page="report"`) {
		t.Fatal("missing Mail overview / Actioned / Report page tabs")
	}
	if !strings.Contains(html, `class="panel page-actioned"`) || strings.Contains(html, `id="sec-actioned" class="panel actioned-board page-overview"`) {
		t.Fatal("Actioned table and charts must sit on the Actioned tab, not Mail overview")
	}
	if !strings.Contains(html, `<h1 id="board-title">should-i-read</h1>`) || !strings.Contains(html, `class="brand-mark"`) {
		t.Fatal("header must keep the should-i-read mark and heading on both pages")
	}
	brandEnd := strings.Index(html, `id="board-title"`)
	pageIdx := strings.Index(html, `class="page-tabs"`)
	actionsIdx := strings.Index(html, `class="header-actions"`)
	monthIdx := strings.Index(html, `class="month-tabs"`)
	whenIdx := strings.Index(html, `id="mail-sync-when"`)
	if brandEnd < 0 || pageIdx < 0 || actionsIdx < 0 || monthIdx < 0 || whenIdx < 0 {
		t.Fatal("missing header brand, page tabs, actions, months, or Sync time")
	}
	if brandEnd >= pageIdx || pageIdx >= actionsIdx || actionsIdx >= whenIdx || whenIdx >= monthIdx {
		t.Fatal("Mail overview tabs must sit right of the logo; generated time must sit in Sync")
	}
	if strings.Contains(html, `class="brand"`) {
		brandChunk := html[strings.Index(html, `class="brand"`):pageIdx]
		if strings.Contains(brandChunk, "mail-sync-when") {
			t.Fatal("generated time must not sit under the product name")
		}
	}
	if strings.Contains(html, `boardTitle.textContent`) || strings.Contains(html, `document.title = page`) {
		t.Fatal("page tabs must not rename the product heading")
	}
	if !strings.Contains(html, `id="sec-actioned-report"`) || !strings.Contains(html, "function paintActionedCharts") {
		t.Fatal("Actioned tab must chart by outcome and sender")
	}
	if !strings.Contains(html, `id="sec-provenance"`) || !strings.Contains(html, `id="sec-provenance" class="panel page-report"`) {
		t.Fatal("Provenance must sit on Report, not a page-wide footer")
	}
	if strings.Contains(html, `class="site-footer"`) || strings.Contains(html, "Definitions") || strings.Contains(html, "Hop savings") || strings.Contains(html, "Fluff tax") {
		t.Fatal("footer glossary must not render")
	}
	if strings.Contains(html, `id="highlight-rect"`) || strings.Contains(html, `id="highlight-toggle"`) {
		t.Fatal("highlight drag box should be gone")
	}
}

func TestRenderHTML_intent_panel(t *testing.T) {
	html := mustRenderHTML(t, sampleBoardModel())
	if !strings.Contains(html, `id="intent-panel"`) || !strings.Contains(html, "function openActionRow") {
		t.Fatal("Reply and Decide rows must open a message preview")
	}
	if !strings.Contains(html, "sir-mail-overview-nav") || !strings.Contains(html, "function restoreNav") {
		t.Fatal("overview must remember page, month, and scroll in localStorage")
	}
	if strings.Contains(html, "Asking Polypus") || !strings.Contains(html, "function showWait") || !strings.Contains(html, "Summarizing this sender") {
		t.Fatal("intent wait must use a spinner, not Asking Polypus")
	}
	if !strings.Contains(html, `id="intent-worth"`) || !strings.Contains(html, `id="intent-mark"`) || !strings.Contains(html, `id="delete-cart"`) {
		t.Fatal("missing worth-reading checkbox, mark, or cart")
	}
	if !strings.Contains(html, `id="intent-outliers-wrap"`) || !strings.Contains(html, "function paintOutlierBullets") {
		t.Fatal("missing outlier bullets under TLDR")
	}
	if !strings.Contains(html, "function openOutlierCard") || !strings.Contains(html, "outlier-card") {
		t.Fatal("priority cards must open a message preview")
	}
	if !strings.Contains(html, "function parseOutlierSubject") || !strings.Contains(html, "outlier-repo") {
		t.Fatal("outlier cards must cluster by repo and drop SHA restatements")
	}
	if !strings.Contains(html, "function dismissPanel") {
		t.Fatal("Mark as deletable should close the panel after a cart post")
	}
	if !strings.Contains(html, "function onOutsideIntent") {
		t.Fatal("clicking outside the sender panel must close it")
	}
	worthIdx := strings.Index(html, `Could not save worth reading.`)
	if worthIdx < 0 {
		t.Fatal("missing worth-reading save error copy")
	}
	worthChunk := html[worthIdx:]
	if end := strings.Index(worthChunk, ".catch(function () {"); end >= 0 {
		worthChunk = worthChunk[:end]
	}
	if strings.Contains(worthChunk, "dismissPanel()") {
		t.Fatal("worth reading must not dismiss the panel")
	}
	if strings.Contains(html, "max-height: min(40vh, 22rem)") || !strings.Contains(html, "max-height: min(82vh, calc(100vh - 5rem))") {
		t.Fatal("message panel should be tall enough to read")
	}
	if !strings.Contains(html, `#intent-panel:not([hidden])`) || !strings.Contains(html, `#intent-panel[hidden]`) {
		t.Fatal("Close must be able to hide the message panel")
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
	if !strings.Contains(html, ".cart-folder-when") || !strings.Contains(html, ".cart-list .cart-when") {
		t.Fatal("missing cart mailed-at situating")
	}
	if strings.Contains(html, "Start deletion job") || strings.Contains(html, "id=\"cart-job-start\"") {
		t.Fatal("checkout should not keep a Start button")
	}
}

func TestRenderHTML_apply_cart_and_actioned(t *testing.T) {
	html := mustRenderHTML(t, sampleBoardModel())
	if !strings.Contains(html, "Apply at source") || !strings.Contains(html, "trash-at-source") || !strings.Contains(html, "seen-at-source") {
		t.Fatal("missing combined Apply at source checkout")
	}
	if !strings.Contains(html, `id="apply-source"`) || !strings.Contains(html, "function applyAtSource") {
		t.Fatal("missing Apply at source header control")
	}
	if !strings.Contains(html, `aria-label="Apply at source"`) || !strings.Contains(html, `class="apply-source-icon"`) {
		t.Fatal("Apply at source must be an icon-only header button")
	}
	if strings.Contains(html, `class="apply-source-label"`) || strings.Contains(html, "applySourceBtn.textContent") {
		t.Fatal("Apply at source must not show a text label on the button")
	}
	if strings.Contains(html, "Move to Trash at source") || strings.Contains(html, "Mark as read at source") {
		t.Fatal("Acked and Cart must share one Apply at source button")
	}
	if !strings.Contains(html, "Checking out deletion cart…") || !strings.Contains(html, "/cart/checkout") {
		t.Fatal("missing cart checkout in-progress copy")
	}
	if !strings.Contains(html, "already in Trash") {
		t.Fatal("missing already in Trash job copy")
	}
	if strings.Contains(html, "Cart has newer marks") {
		t.Fatal("checkout must not split Start vs Move for newer marks")
	}
	if !strings.Contains(html, `id="sec-actioned"`) || !strings.Contains(html, "/actioned") {
		t.Fatal("missing actioned strip")
	}
	if !strings.Contains(html, `is-actioned`) {
		t.Fatal("missing live-board actioned hide")
	}
	if !strings.Contains(html, "function syncLiveChrome") || !strings.Contains(html, "[data-hash].is-actioned") {
		t.Fatal("trashed mail must recount tiles, tabs, and the kind tree")
	}
	if !strings.Contains(html, "lastActioned[h]") {
		t.Fatal("kind-tree clicks must skip hashes already in Trash before the cart cap")
	}
	if !strings.Contains(html, `is-staged`) || !strings.Contains(html, "Staged in the deletion cart") {
		t.Fatal("missing staged-in-cart board mark")
	}
	if strings.Contains(html, `id="cart-job-trash"`) || strings.Contains(html, `id="ack-job-seen"`) {
		t.Fatal("per-cart checkout buttons must be gone")
	}
	if !strings.Contains(html, "<th>Messages</th>") || !strings.Contains(html, "<td>2</td>") {
		t.Fatal("missing sender message counts")
	}
	if !strings.Contains(html, "--highlight-fill:") {
		t.Fatal("missing transparent highlight fill token")
	}
}

func TestRenderHTML_headerSync(t *testing.T) {
	m := mailoverview.Model{
		SchemaVersion: mailoverview.SchemaVersion,
		GeneratedAt:   "2025-01-01T00:00:00Z",
		All: mailoverview.MonthSection{
			Key: "all", Label: "All",
			Summary: mailoverview.SummaryStats{Classified: 1},
		},
	}
	html, err := RenderHTML(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `id="mail-sync"`) || !strings.Contains(html, "/sync") {
		t.Fatal("missing header sync control")
	}
	if !strings.Contains(html, `class="mail-sync-spinner"`) || !strings.Contains(html, `class="mail-sync-label"`) {
		t.Fatal("sync button must include label and in-button spinner")
	}
	if !strings.Contains(html, `id="mail-sync-when"`) || !strings.Contains(html, `data-local="board"`) || !strings.Contains(html, "function localizeTimes") {
		t.Fatal("Sync must show a compact local generated time")
	}
	if !strings.Contains(html, `mail-sync-spin`) {
		t.Fatal("sync spinner keyframes must ship in CSS")
	}
	actionsStart := strings.Index(html, `class="header-actions"`)
	monthStart := strings.Index(html, `class="month-row"`)
	if actionsStart < 0 || monthStart < 0 || actionsStart > monthStart {
		t.Fatal("header-actions must sit above the month row")
	}
	actions := html[actionsStart:monthStart]
	month := html[monthStart:]
	if strings.Contains(actions, `id="ack-toggle"`) || strings.Contains(actions, `id="cart-toggle"`) || strings.Contains(actions, `id="mail-sync-stripe"`) {
		t.Fatal("Acked, Cart, and the sync error stripe do not belong on the title row")
	}
	if !strings.Contains(actions, `id="mail-sync"`) || !strings.Contains(actions, `id="theme-toggle"`) {
		t.Fatal("title row must keep Light / dark and Sync")
	}
	ackIdx := strings.Index(month, `id="ack-toggle"`)
	cartIdx := strings.Index(month, `id="cart-toggle"`)
	if ackIdx < 0 || cartIdx < 0 {
		t.Fatal("month row must keep Acked and Cart")
	}
	if strings.Contains(month, `id="mail-sync-when"`) || strings.Contains(month, "Generated ") {
		t.Fatal("generated time must sit inside Sync, not on the month row")
	}
	if ackIdx >= cartIdx {
		t.Fatal("Acked then Cart must sit together on the month row")
	}
	if !strings.Contains(month, `id="apply-source"`) {
		t.Fatal("Apply at source must sit with Acked and Cart on the month row")
	}
	if !strings.Contains(month, `class="apply-source-icon"`) || !strings.Contains(month, `aria-label="Apply at source"`) {
		t.Fatal("Apply at source must be an icon-only control on the month row")
	}
	stripeIdx := strings.Index(html, `id="mail-sync-stripe"`)
	headerEnd := strings.Index(html, "</header>")
	if stripeIdx < 0 || headerEnd < 0 || stripeIdx < headerEnd {
		t.Fatal("sync error stripe must sit under the sticky site header, not inside it")
	}
	if strings.Contains(html, `id="mail-sync-status"`) {
		t.Fatal("sync errors must not sit on the Generated line")
	}
	if !strings.Contains(html, `.mail-sync-stripe`) || !strings.Contains(html, "#6b1219") {
		t.Fatal("sync error stripe must be a dark red bar")
	}
	if !strings.Contains(html, `.mail-sync-stripe.is-progress`) || !strings.Contains(html, "#243447") {
		t.Fatal("sync progress stripe must be a slate bar")
	}
	if !strings.Contains(html, "isolation: isolate") {
		t.Fatal("site header must isolate so table headers cannot paint over the sync stripe")
	}
	thIdx := strings.Index(html, "table.data th {")
	if thIdx < 0 {
		t.Fatal("missing table header rule")
	}
	thCSS := html[thIdx:]
	if len(thCSS) > 280 {
		thCSS = thCSS[:280]
	}
	if strings.Contains(thCSS, "position: sticky") {
		t.Fatal("table headers must not be sticky; they paint over the sync stripe")
	}
	wrapIdx := strings.Index(html, "\n.table-wrap {")
	if wrapIdx < 0 {
		t.Fatal("missing table-wrap rule")
	}
	wrapCSS := html[wrapIdx:]
	if len(wrapCSS) > 240 {
		wrapCSS = wrapCSS[:240]
	}
	if !strings.Contains(wrapCSS, "position: relative") || !strings.Contains(wrapCSS, "z-index: 0") {
		t.Fatal("table wrap must stay in a stacking context below the site header")
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

func TestRenderHTML_heatmapNoDateWeek(t *testing.T) {
	m := mailoverview.Model{
		SchemaVersion: 1,
		GeneratedAt:   "2025-01-01T00:00:00Z",
		All: mailoverview.MonthSection{
			Key: "all", Label: "All",
			Summary: mailoverview.SummaryStats{Classified: 1},
			Attention: mailoverview.AttentionBlock{
				UrgencyHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
				Donut:       []mailoverview.DonutSlice{{Label: "Quiet", Count: 1}},
			},
			Themes: []mailoverview.ThemeCount{{Slot: "Other", Count: 1}},
			Heatmap: mailoverview.HeatmapData{
				WeekLabels:  []string{mailoverview.NoWeekKey},
				ThemeLabels: []string{"Other"},
				Cells:       [][]int{{1}},
			},
			Trust: mailoverview.TrustBlock{
				CosineHist: mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
				JudgeHist:  mailoverview.Histogram{Buckets: []float64{0, 1}, Counts: []int{1}},
			},
		},
	}
	html, err := RenderHTML(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Week of No date, Other: 1") {
		t.Fatal("heatmap should label the undated week")
	}
	if strings.Contains(html, `text-anchor="middle">(no week)</text>`) {
		t.Fatal("heatmap header should not show the raw no-week key")
	}
}
