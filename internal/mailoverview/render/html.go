package render

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/xynova/should-i-read/internal/mailoverview"
	"github.com/xynova/should-i-read/internal/mailoverview/svg"
)

//go:embed templates/overview.css
var cssBytes []byte

//go:embed templates/overview.js
var jsBytes []byte

//go:embed templates/overview.html.tmpl
var htmlTmpl string

// Chart fills follow the page colour tokens so both themes stay legible.
const (
	chartWarm  = "var(--chart-warm)"
	chartMuted = "var(--chart-muted)"
)

// HTMLAssets holds inlined CSS and JS for tests and embedding.
type HTMLAssets struct {
	CSS string
	JS  string
}

// LoadHTMLAssets returns trimmed CSS and JS text.
func LoadHTMLAssets() HTMLAssets {
	return HTMLAssets{
		CSS: string(bytes.TrimSpace(cssBytes)),
		JS:  string(bytes.TrimSpace(jsBytes)),
	}
}

// minorMonthMax is the largest message count a month tab can hold and still
// be grouped with the stray dates after the main tabs.
const minorMonthMax = 9

// TabView is one month tab plus precomputed SVG strings for that section.
type TabView struct {
	Key   string
	Label string
	// Count is the number of classified messages in the tab.
	Count int
	// Minor is true for the unknown month and months with few messages.
	Minor bool
	// Selected is true for the latest main calendar month with mail, else All.
	Selected bool

	Section     mailoverview.MonthSection
	Tiles       []AttentionTile
	ActionLists []ActionList
	HasThemes   bool
	HasFluff    bool

	SparklineSVG      template.HTML
	AttentionDonutSVG template.HTML
	UrgencyHistSVG    template.HTML
	HeatmapSVG        template.HTML
	// KindTree is the folded top kind, area and message type tree.
	KindTree         *KindNode
	FluffWeekBarsSVG template.HTML
	FluffSplitSVG    template.HTML
	HopFlowSVG       template.HTML
	SkipRateSVG      template.HTML
	CosineHistSVG    template.HTML
	JudgeHistSVG     template.HTML

	UrgencyBucketRows []bucketRow
	CosineBucketRows  []bucketRow
	JudgeBucketRows   []bucketRow
}

type bucketRow struct {
	Label string
	Count int
}

type pageData struct {
	Model     mailoverview.Model
	ModelJSON template.JS
	Tabs      []TabView
	// MainTabs and MinorTabs split Tabs for the tab bar.
	MainTabs  []TabView
	MinorTabs []TabView
	Generated string
	InlineCSS template.CSS
	InlineJS  template.JS

	DeleteSec    int
	KeepExtraSec int
	UrgThreshold float64

	// Definitions and print strings
}

// RenderHTML writes a standalone page.
func RenderHTML(m mailoverview.Model) (string, error) {
	assets := LoadHTMLAssets()
	tmpl, err := template.New("overview").Parse(htmlTmpl)
	if err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	tabs := buildTabs(m)
	pd := pageData{
		Model:        m,
		ModelJSON:    template.JS(raw),
		Tabs:         tabs,
		Generated:    generatedLabel(m.GeneratedAt),
		InlineCSS:    template.CSS(assets.CSS),
		InlineJS:     template.JS(assets.JS),
		DeleteSec:    5,
		KeepExtraSec: 10,
		UrgThreshold: 0.5,
	}
	for _, t := range tabs {
		if t.Minor {
			pd.MinorTabs = append(pd.MinorTabs, t)
		} else {
			pd.MainTabs = append(pd.MainTabs, t)
		}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, pd); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func buildTabs(m mailoverview.Model) []TabView {
	refYear := yearOf(m.GeneratedAt)
	views := make([]TabView, 0, len(m.Months)+2)
	views = append(views, sectionView("all", m.All.Label, m.All, refYear))
	for _, ms := range m.Months {
		v := sectionView(ms.Key, ms.Label, ms, refYear)
		v.Minor = ms.Key == "unknown" || ms.Summary.Classified <= minorMonthMax
		views = append(views, v)
	}
	sel := defaultTabKey(views)
	for i := range views {
		views[i].Selected = views[i].Key == sel
	}
	return views
}

func sectionView(key, label string, sec mailoverview.MonthSection, refYear int) TabView {
	v := TabView{
		Key:         key,
		Label:       label,
		Count:       sec.Summary.Classified,
		Section:     sec,
		Tiles:       attentionTiles(sec.Attention),
		ActionLists: actionLists(sec.Attention, refYear),
		HasThemes:   len(sec.Themes) > 0 || len(sec.KindTree.Paths) > 0,
		HasFluff:    len(sec.Fluff.ByWeek) > 0 || len(sec.Fluff.TopSenders) > 0,
	}
	// Sparkline: need at least two points
	if len(sec.Summary.Sparkline) >= 2 {
		v.SparklineSVG = template.HTML(svg.Sparkline(520, 120, "Classified by month (6 months)", sec.Summary.Sparkline, sec.Summary.SparkMonths))
	}
	// Donut
	var labs []string
	var counts []int
	colors := []string{"#6b7280", "#c27a00", "#2a6fdb", "#1f8a4c"}
	for _, sl := range sec.Attention.Donut {
		labs = append(labs, sl.Label)
		counts = append(counts, sl.Count)
	}
	if len(labs) > 0 {
		v.AttentionDonutSVG = template.HTML(svg.DonutChart(220, "Attention mix", labs, counts, colors))
	}
	// Urgency histogram
	h := sec.Attention.UrgencyHist
	for i := range h.Counts {
		if i+1 >= len(h.Buckets) {
			break
		}
		v.UrgencyBucketRows = append(v.UrgencyBucketRows, bucketRow{
			Label: fmt.Sprintf("%.2f-%.2f", h.Buckets[i], h.Buckets[i+1]),
			Count: h.Counts[i],
		})
	}
	v.UrgencyHistSVG = template.HTML(svg.HistogramBars(480, 160, "Urgency distribution", "bucket (0-1)", "classify_state", h.Buckets, h.Counts, h.Threshold, "threshold 0.50", "currentColor"))

	ch := sec.Trust.CosineHist
	for i := range ch.Counts {
		if i+1 >= len(ch.Buckets) {
			break
		}
		v.CosineBucketRows = append(v.CosineBucketRows, bucketRow{
			Label: fmt.Sprintf("%.2f-%.2f", ch.Buckets[i], ch.Buckets[i+1]),
			Count: ch.Counts[i],
		})
	}
	jh := sec.Trust.JudgeHist
	for i := range jh.Counts {
		if i+1 >= len(jh.Buckets) {
			break
		}
		v.JudgeBucketRows = append(v.JudgeBucketRows, bucketRow{
			Label: fmt.Sprintf("%.2f-%.2f", jh.Buckets[i], jh.Buckets[i+1]),
			Count: jh.Counts[i],
		})
	}

	// Type totals live in the heatmap's Total column, not a separate chart.
	hm := sec.Heatmap
	if len(hm.WeekLabels) > 0 && len(hm.ThemeLabels) > 0 {
		weeks := make([]string, len(hm.WeekLabels))
		for i, w := range hm.WeekLabels {
			weeks[i] = weekLabel(w)
		}
		themes := make([]string, len(hm.ThemeLabels))
		for i, t := range hm.ThemeLabels {
			themes[i] = humanLabel(t)
		}
		v.HeatmapSVG = template.HTML(svg.HeatmapGrid(1100, 76+float64(len(themes))*28, "Message types by week (week starting Monday)", "Darker cells hold more messages. Types are the last word of each kind path; Other holds every type not listed.", weeks, themes, hm.Cells, hm.CellHashes, hm.ThemeHashes))
	}
	v.KindTree = buildKindTree(sec.KindTree.Paths)

	var wkLab []string
	var wkVal []int
	for _, w := range sec.Fluff.ByWeek {
		wkLab = append(wkLab, "Week of "+weekLabel(w.WeekKey))
		wkVal = append(wkVal, int(w.Minutes*60))
	}
	if len(wkLab) > 0 {
		v.FluffWeekBarsSVG = template.HTML(svg.BarChart(520, 64+float64(len(wkLab))*26, "Triage time by week (seconds)", "seconds", "Counts 5 s to delete each message, plus 10 s when it needs a reply or decision.", wkLab, wkVal, chartWarm))
	}
	messagesN := sec.Summary.Classified
	if messagesN < 1 {
		messagesN = 1
	}
	delSec := messagesN * 5
	keepSec := int(sec.Fluff.KeepMinutes * 60)
	v.FluffSplitSVG = template.HTML(svg.StackedBar(520, 100, "Deleting versus acting", "Deleting (5 s each)", "Acting (10 s extra when flagged)", delSec, keepSec, chartMuted, chartWarm))

	// Hop flow
	var toJev, toEss, jevOut, essOut int
	for _, e := range sec.Pipeline.HopFlow.Edges {
		switch e.From + "->" + e.To {
		case "Messages->Jev_skip":
			toJev = e.Count
		case "Messages->Essence":
			toEss = e.Count
		case "Jev_skip->Classified":
			jevOut = e.Count
		case "Essence->Classified":
			essOut = e.Count
		}
	}
	v.HopFlowSVG = template.HTML(svg.HopFlowSVG(440, 180, "Pipeline hop counts", toJev, toEss, jevOut, essOut))

	if len(sec.Pipeline.SkipRateWeek) > 1 {
		var labs []string
		var rates []float64
		for _, w := range sec.Pipeline.SkipRateWeek {
			labs = append(labs, w.Label)
			rates = append(rates, w.Rate)
		}
		v.SkipRateSVG = template.HTML(svg.LineSeries(520, 160, "Jev skip rate by week", "rate 0-1", labs, rates))
	}

	v.CosineHistSVG = template.HTML(svg.HistogramBars(480, 150, "Cosine similarity", "bucket", "ledger", sec.Trust.CosineHist.Buckets, sec.Trust.CosineHist.Counts, 0, "", "currentColor"))
	v.JudgeHistSVG = template.HTML(svg.HistogramBars(480, 150, "Judge score", "bucket", "ledger", sec.Trust.JudgeHist.Buckets, sec.Trust.JudgeHist.Counts, 0, "", "currentColor"))

	return v
}
