package svg

import (
	"fmt"
	"math"
	"strings"
)

// BarChart renders a horizontal bar chart; values must be non-negative.
// The x label and caption may be empty, and the colour may be any CSS colour,
// including var().
func BarChart(width, height float64, title, xLabel, caption string, categories []string, values []int, color string) string {
	if len(categories) != len(values) || len(categories) == 0 {
		return ""
	}
	maxV := 1
	longest := 0
	for i, v := range values {
		if v > maxV {
			maxV = v
		}
		if n := len([]rune(categories[i])); n > longest {
			longest = n
		}
	}
	marginL := math.Min(math.Max(float64(longest)*6.6+16, 72), 190)
	marginR := 48.0
	marginT := 34.0
	marginB := 8.0
	if xLabel != "" {
		marginB += 16
	}
	if caption != "" {
		marginB += 16
	}
	plotW := width - marginL - marginR
	rowH := (height - marginT - marginB) / float64(len(categories))
	if rowH < 16 {
		rowH = 16
	}
	barH := math.Min(rowH-8, 18)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, width, height, width, height)
	fmt.Fprintf(&b, `<title>%s</title>`, escape(title))
	fmt.Fprintf(&b, `<text x="0" y="16" fill="currentColor" font-size="13" font-weight="600">%s</text>`, escape(title))
	for i, cat := range categories {
		v := values[i]
		w := math.Max(float64(v)/float64(maxV)*plotW, 2)
		y := marginT + float64(i)*rowH
		mid := y + rowH/2
		fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" fill="currentColor" font-size="12" text-anchor="end" dominant-baseline="middle">%s</text>`, marginL-10, mid, escape(truncate(cat, 28)))
		fmt.Fprintf(&b, `<rect x="%.0f" y="%.1f" width="%.1f" height="%.1f" rx="3" style="fill:%s">`, marginL, mid-barH/2, w, barH, color)
		fmt.Fprintf(&b, `<title>%s: %d</title></rect>`, escape(cat), v)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="currentColor" font-size="12" font-weight="600" dominant-baseline="middle">%d</text>`, marginL+w+6, mid, v)
	}
	y := height - 8
	if caption != "" {
		fmt.Fprintf(&b, `<text x="0" y="%.0f" fill="currentColor" font-size="11" opacity="0.7">%s</text>`, y, escape(caption))
		y -= 16
	}
	if xLabel != "" {
		fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" fill="currentColor" font-size="11" opacity="0.7">%s</text>`, marginL, y, escape(xLabel))
	}
	fmt.Fprintf(&b, `</svg>`)
	return b.String()
}

// StackedBar is one row with two segments (counts a and b).
func StackedBar(width, height float64, title, aLabel, bLabel string, a, b int, colorA, colorB string) string {
	total := a + b
	if total == 0 {
		total = 1
	}
	barW := width
	barH := 22.0
	wa := float64(a) / float64(total) * barW
	wb := float64(b) / float64(total) * barW
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, width, height, width, height)
	fmt.Fprintf(&sb, `<title>%s</title>`, escape(title))
	fmt.Fprintf(&sb, `<text x="0" y="16" fill="currentColor" font-size="13" font-weight="600">%s</text>`, escape(title))
	fmt.Fprintf(&sb, `<rect x="0" y="32" width="%.1f" height="%.0f" style="fill:%s"><title>%s %d</title></rect>`, wa, barH, colorA, escape(aLabel), a)
	fmt.Fprintf(&sb, `<rect x="%.1f" y="32" width="%.1f" height="%.0f" style="fill:%s"><title>%s %d</title></rect>`, wa, wb, barH, colorB, escape(bLabel), b)
	fmt.Fprintf(&sb, `<rect x="0" y="72" width="10" height="10" rx="2" style="fill:%s"/><text x="16" y="81" fill="currentColor" font-size="11">%s: %d s</text>`, colorA, escape(aLabel), a)
	fmt.Fprintf(&sb, `<rect x="%.0f" y="72" width="10" height="10" rx="2" style="fill:%s"/><text x="%.0f" y="81" fill="currentColor" font-size="11">%s: %d s</text>`, width/2, colorB, width/2+16, escape(bLabel), b)
	fmt.Fprintf(&sb, `</svg>`)
	return sb.String()
}

// DonutChart renders a donut from parallel label/count slices.
func DonutChart(size float64, title string, labels []string, counts []int, colors []string) string {
	if len(labels) != len(counts) {
		return ""
	}
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		total = 1
	}
	cx, cy := size/2, size/2+20
	r := size * 0.28
	rin := r * 0.55
	var parts strings.Builder
	fmt.Fprintf(&parts, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, size, size+28, size, size+28)
	fmt.Fprintf(&parts, `<title>%s</title>`, escape(title))
	fmt.Fprintf(&parts, `<text x="8" y="16" fill="currentColor" font-size="12">%s</text>`, escape(title))
	angle := -90.0
	for i := range labels {
		frac := float64(counts[i]) / float64(total)
		a1 := angle
		a2 := angle + frac*360
		angle = a2
		path := donutSegment(cx, cy, r, rin, a1, a2)
		col := "#888"
		if i < len(colors) {
			col = colors[i]
		}
		fmt.Fprintf(&parts, `<path d="%s" fill="%s"><title>%s: %d</title></path>`, path, col, escape(labels[i]), counts[i])
	}
	fmt.Fprintf(&parts, `</svg>`)
	return parts.String()
}

func donutSegment(cx, cy, r, rin, a1, a2 float64) string {
	x1 := cx + r*math.Cos(rad(a1))
	y1 := cy + r*math.Sin(rad(a1))
	x2 := cx + r*math.Cos(rad(a2))
	y2 := cy + r*math.Sin(rad(a2))
	x3 := cx + rin*math.Cos(rad(a2))
	y3 := cy + rin*math.Sin(rad(a2))
	x4 := cx + rin*math.Cos(rad(a1))
	y4 := cy + rin*math.Sin(rad(a1))
	large := 0
	if a2-a1 > 180 {
		large = 1
	}
	return fmt.Sprintf("M %.2f %.2f A %.2f %.2f 0 %d 1 %.2f %.2f L %.2f %.2f A %.2f %.2f 0 %d 0 %.2f %.2f Z",
		x1, y1, r, r, large, x2, y2, x3, y3, rin, rin, large, x4, y4)
}

func rad(a float64) float64 {
	return a * math.Pi / 180
}

// HistogramBars renders vertical bars for bucket counts; buckets are lo edges, len(counts)==len(buckets)-1.
func HistogramBars(width, height float64, title, xLabel, caption string, buckets []float64, counts []int, refX float64, refLabel string, barColor string) string {
	if len(buckets) < 2 || len(counts) != len(buckets)-1 {
		return ""
	}
	maxC := 1
	for _, c := range counts {
		if c > maxC {
			maxC = c
		}
	}
	marginL := 40.0
	marginR := 16.0
	marginT := 32.0
	marginB := 44.0
	plotW := width - marginL - marginR
	plotH := height - marginT - marginB
	n := float64(len(counts))
	bw := plotW / n
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, width, height, width, height)
	fmt.Fprintf(&b, `<title>%s</title>`, escape(title))
	fmt.Fprintf(&b, `<text x="8" y="18" fill="currentColor" font-size="12">%s</text>`, escape(title))
	// reference vertical line
	span := buckets[len(buckets)-1] - buckets[0]
	if refLabel != "" && span > 0 && refX >= buckets[0] && refX <= buckets[len(buckets)-1] {
		rx := marginL + (refX-buckets[0])/span*plotW
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="currentColor" stroke-dasharray="4 3" opacity="0.6"><title>%s</title></line>`, rx, marginT, rx, marginT+plotH, escape(refLabel))
	}
	for i, c := range counts {
		h := float64(c) / float64(maxC) * plotH
		x := marginL + float64(i)*bw + bw*0.12
		ww := bw * 0.76
		y := marginT + plotH - h
		lo := buckets[i]
		hi := buckets[i+1]
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s">`, x, y, ww, h, barColor)
		fmt.Fprintf(&b, `<title>[%.2f,%.2f): %d</title></rect>`, lo, hi, c)
	}
	fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" fill="currentColor" font-size="9" text-anchor="middle">%s</text>`, marginL+plotW/2, height-12, escape(xLabel))
	fmt.Fprintf(&b, `<text x="8" y="%.0f" fill="currentColor" font-size="10" opacity="0.75">%s</text>`, height-2, escape(caption))
	fmt.Fprintf(&b, `</svg>`)
	return b.String()
}

// HeatmapGrid renders week x theme counts, with cells indexed [week][theme].
// Cells shade from faint to the chart colour; numbers sit inside each cell.
// A Total column after the weeks shows each row's sum as a bar and a number.
// CellHashes and ThemeHashes are optional comma-joined object hashes, parallel
// to cells and theme labels.
func HeatmapGrid(width, height float64, title, caption string, weekLabels, themeLabels []string, cells [][]int, cellHashes [][]string, themeHashes []string) string {
	if len(weekLabels) == 0 || len(themeLabels) == 0 {
		return ``
	}
	maxC := 1
	for _, row := range cells {
		for _, c := range row {
			if c > maxC {
				maxC = c
			}
		}
	}
	longest := 0
	for _, t := range themeLabels {
		if n := len([]rune(t)); n > longest {
			longest = n
		}
	}
	totals := make([]int, len(themeLabels))
	maxT := 1
	for ti := range themeLabels {
		for wi := range weekLabels {
			if wi < len(cells) && ti < len(cells[wi]) {
				totals[ti] += cells[wi][ti]
			}
		}
		if totals[ti] > maxT {
			maxT = totals[ti]
		}
	}
	const totalGap, totalBarMax, totalW = 24.0, 150.0, 210.0
	marginL := math.Min(math.Max(float64(longest)*6.6+16, 72), 190)
	marginT := 52.0
	marginB := 26.0
	nw := float64(len(weekLabels))
	cellW := math.Min((width-marginL-totalGap-totalW)/nw, 120)
	cellH := (height - marginT - marginB) / float64(len(themeLabels))
	totalX := marginL + cellW*nw + totalGap
	svgW := totalX + totalW
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, svgW, height, svgW, height)
	fmt.Fprintf(&b, `<title>%s</title>`, escape(title))
	fmt.Fprintf(&b, `<text x="0" y="16" fill="currentColor" font-size="13" font-weight="600">%s</text>`, escape(title))
	for wi, wk := range weekLabels {
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="currentColor" font-size="11" opacity="0.75" text-anchor="middle">%s</text>`,
			marginL+float64(wi)*cellW+cellW/2, marginT-10, escape(truncate(wk, 12)))
	}
	for ti, th := range themeLabels {
		fmt.Fprintf(&b, `<text x="%.0f" y="%.1f" fill="currentColor" font-size="12" text-anchor="end" dominant-baseline="middle"%s>%s</text>`, marginL-10, marginT+float64(ti)*cellH+cellH/2, dataHashesAttr(hashAt(themeHashes, ti)), escape(truncate(th, 28)))
	}
	for wi := range weekLabels {
		for ti := range themeLabels {
			c := 0
			if wi < len(cells) && ti < len(cells[wi]) {
				c = cells[wi][ti]
			}
			x := marginL + float64(wi)*cellW
			y := marginT + float64(ti)*cellH
			t := float64(c) / float64(maxC)
			fmt.Fprintf(&b, `<g%s>`, dataHashesAttr(cellHashAt(cellHashes, wi, ti)))
			if c == 0 {
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="3" fill="currentColor" fill-opacity="0.05">`, x+2, y+2, cellW-4, cellH-4)
			} else {
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="3" style="fill:var(--chart-strong)" fill-opacity="%.2f">`, x+2, y+2, cellW-4, cellH-4, 0.15+0.85*t)
			}
			fmt.Fprintf(&b, `<title>Week of %s, %s: %d</title></rect>`, escape(weekLabels[wi]), escape(themeLabels[ti]), c)
			if c > 0 {
				// Pastel cells stay light enough for the page text colour.
				fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" style="fill:var(--text)" font-size="11" text-anchor="middle" dominant-baseline="middle">%d</text>`, x+cellW/2, y+cellH/2, c)
			}
			fmt.Fprintf(&b, `</g>`)
		}
	}
	fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="currentColor" font-size="11" font-weight="600" opacity="0.85">Total</text>`, totalX, marginT-10)
	for ti, tot := range totals {
		y := marginT + float64(ti)*cellH
		w := math.Max(float64(tot)/float64(maxT)*totalBarMax, 2)
		barH := math.Min(cellH-10, 14)
		fmt.Fprintf(&b, `<g%s>`, dataHashesAttr(hashAt(themeHashes, ti)))
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="3" style="fill:var(--chart)">`, totalX, y+cellH/2-barH/2, w, barH)
		fmt.Fprintf(&b, `<title>%s, all weeks shown: %d</title></rect>`, escape(themeLabels[ti]), tot)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="currentColor" font-size="12" font-weight="600" dominant-baseline="middle">%d</text>`, totalX+w+6, y+cellH/2, tot)
		fmt.Fprintf(&b, `</g>`)
	}
	fmt.Fprintf(&b, `<text x="0" y="%.0f" fill="currentColor" font-size="11" opacity="0.7">%s</text>`, height-6, escape(caption))
	fmt.Fprintf(&b, `</svg>`)
	return b.String()
}

// Sparkline polyline from integer series (at least 2 points).
func Sparkline(width, height float64, title string, values []int, labels []string) string {
	if len(values) < 2 {
		return ""
	}
	maxV := 1
	for _, v := range values {
		if v > maxV {
			maxV = v
		}
	}
	margin := 28.0
	plotW := width - 2*margin
	plotH := height - 2*margin
	var pts []string
	den := float64(len(values) - 1)
	for i, v := range values {
		x := margin + plotW*float64(i)/den
		y := margin + plotH - float64(v)/float64(maxV)*plotH
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", x, y))
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, width, height, width, height)
	fmt.Fprintf(&b, `<title>%s</title>`, escape(title))
	fmt.Fprintf(&b, `<text x="8" y="16" fill="currentColor" font-size="11">%s</text>`, escape(title))
	fmt.Fprintf(&b, `<polyline fill="none" stroke="currentColor" stroke-width="2" points="%s"/>`, strings.Join(pts, " "))
	for i := range values {
		if i < len(labels) {
			x := margin + plotW*float64(i)/den
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="currentColor" font-size="7" text-anchor="middle">%s</text>`, x, height-8, truncate(labels[i], 7))
		}
	}
	fmt.Fprintf(&b, `</svg>`)
	return b.String()
}

// LineSeries draws one line with x from 0..n-1 and y in 0..1 (rates).
func LineSeries(width, height float64, title, caption string, labels []string, rates []float64) string {
	if len(rates) != len(labels) || len(rates) == 0 {
		return ""
	}
	margin := 36.0
	plotW := width - 2*margin
	plotH := height - 2*margin
	den := float64(len(rates) - 1)
	if den < 1 {
		den = 1
	}
	var pts []string
	for i, r := range rates {
		if r < 0 {
			r = 0
		}
		if r > 1 {
			r = 1
		}
		x := margin + plotW*float64(i)/den
		y := margin + plotH - r*plotH
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", x, y))
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, width, height, width, height)
	fmt.Fprintf(&b, `<title>%s</title>`, escape(title))
	fmt.Fprintf(&b, `<text x="8" y="18" fill="currentColor" font-size="12">%s</text>`, escape(title))
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="currentColor" opacity="0.25"/>`, margin, margin+plotH, margin+plotW, margin+plotH)
	fmt.Fprintf(&b, `<polyline fill="none" stroke="currentColor" stroke-width="2" points="%s"/>`, strings.Join(pts, " "))
	for i := range rates {
		x := margin + plotW*float64(i)/den
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="currentColor" font-size="7" text-anchor="middle">%s</text>`, x, height-10, truncate(labels[i], 8))
	}
	fmt.Fprintf(&b, `<text x="8" y="%.0f" fill="currentColor" font-size="10" opacity="0.75">%s</text>`, height-2, escape(caption))
	fmt.Fprintf(&b, `</svg>`)
	return b.String()
}

// HopFlowSVG is a three-column flow: in, middle, out with labeled edge counts.
func HopFlowSVG(width, height float64, title string, toJev, toEssence, fromJev, fromEssence int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img">`, width, height, width, height)
	fmt.Fprintf(&b, `<title>%s</title>`, escape(title))
	fmt.Fprintf(&b, `<text x="8" y="18" fill="currentColor" font-size="12">%s</text>`, escape(title))
	cols := []struct{ x, label string }{{"60", "Messages"}, {"220", "Route"}, {"380", "Classified"}}
	for _, c := range cols {
		fmt.Fprintf(&b, `<text x="%s" y="80" fill="currentColor" font-size="11" text-anchor="middle">%s</text>`, c.x, c.label)
	}
	// edges
	fmt.Fprintf(&b, `<path d="M 100 100 L 200 55" stroke="currentColor" fill="none"/><text x="130" y="72" font-size="10" fill="currentColor">Jev %d</text>`, toJev)
	fmt.Fprintf(&b, `<path d="M 100 115 L 200 145" stroke="currentColor" fill="none"/><text x="130" y="138" font-size="10" fill="currentColor">Essence %d</text>`, toEssence)
	fmt.Fprintf(&b, `<path d="M 240 55 L 340 100" stroke="currentColor" fill="none"/><text x="275" y="72" font-size="10" fill="currentColor">%d</text>`, fromJev)
	fmt.Fprintf(&b, `<path d="M 240 145 L 340 100" stroke="currentColor" fill="none"/><text x="275" y="138" font-size="10" fill="currentColor">%d</text>`, fromEssence)
	fmt.Fprintf(&b, `</svg>`)
	return b.String()
}

func dataHashesAttr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return ` data-hashes="` + escape(s) + `"`
}

func hashAt(hashes []string, i int) string {
	if i < 0 || i >= len(hashes) {
		return ""
	}
	return hashes[i]
}

func cellHashAt(cellHashes [][]string, wi, ti int) string {
	if wi < 0 || wi >= len(cellHashes) {
		return ""
	}
	row := cellHashes[wi]
	if ti < 0 || ti >= len(row) {
		return ""
	}
	return row[ti]
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `<`, `&lt;`)
	s = strings.ReplaceAll(s, `>`, `&gt;`)
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	return s
}

func truncate(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}
