package mailoverview

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/pimdir"
)

const (
	deleteFluffSecs = 5
	keepFluffSecs   = 15
	hopsPerSkip     = 3
	urgThreshold    = 0.5
)

// enriched joins one ledger row with optional mail metadata.
type enriched struct {
	row     hoststore.OverviewRow
	summary pimdir.EmailSummary
	hasMail bool
	month   string
	week    time.Time
	weekKey string
	theme   string
	email   string
	name    string
}

// BuildModel builds the full overview model from ledger rows and mail summaries.
func BuildModel(
	now time.Time,
	loc *time.Location,
	in Input,
	rows []hoststore.OverviewRow,
	summaries map[string]pimdir.EmailSummary,
	ledgerPath, mailStorePath, version string,
	gitRevision string,
	shapes []hoststore.MailShapeRow,
	errRows []hoststore.ErrorRow,
) Model {
	if loc == nil {
		loc = time.Local
	}
	var items []enriched
	for _, r := range rows {
		sum, ok := summaries[r.ObjectHash]
		month := "unknown"
		var wk time.Time
		if ok {
			if t, ok := ParseMailDate(sum.Date, loc); ok {
				month = t.Format("2006-01")
				wk = MondayWeekStart(t, loc)
			}
		}
		wkey := ""
		if !wk.IsZero() {
			wkey = wk.Format("2006-01-02")
		}
		email := strings.TrimSpace(sum.Sender)
		if email == "" {
			email = "(unknown sender)"
		}
		theme := ThemeCategory(lastKindSegment(r.Kind))
		items = append(items, enriched{
			row:     r,
			summary: sum,
			hasMail: ok,
			month:   month,
			week:    wk,
			weekKey: wkey,
			theme:   theme,
			email:   email,
			name:    strings.Trim(strings.TrimSpace(sum.SenderName), `"'`),
		})
	}

	monthSet := map[string]struct{}{}
	for _, it := range items {
		if it.month != "" {
			monthSet[it.month] = struct{}{}
		}
	}
	var monthKeys []string
	for m := range monthSet {
		if inRangeMonth(m, in.SinceMonth, in.UntilMonth) && m != "unknown" {
			monthKeys = append(monthKeys, m)
		}
	}
	sort.Strings(monthKeys)

	seriesByMonth := map[string]int{}
	for _, it := range items {
		if it.month != "unknown" {
			seriesByMonth[it.month]++
		}
	}
	sparkMonths := lastNMonthKeys(now, loc, 6)

	var sections []MonthSection
	for _, mk := range monthKeys {
		flt := filterMonth(items, mk)
		prevKey := prevCalendarMonthKey(mk)
		sections = append(sections, buildMonthSection(mk, monthLabel(mk), flt, prevKey, sparkMonths, seriesByMonth, in))
	}
	if unknown := filterMonth(items, "unknown"); len(unknown) > 0 && inRangeMonth("unknown", in.SinceMonth, in.UntilMonth) {
		sections = append(sections, buildMonthSection("unknown", "Unknown month", unknown, "", sparkMonths, seriesByMonth, in))
	}
	allSec := buildMonthSection("all", "All", items, "", sparkMonths, seriesByMonth, in)

	shapeOut := make([]ShapeInfo, 0, len(shapes))
	promptHashes := map[string]int{}
	for _, s := range shapes {
		shapeOut = append(shapeOut, ShapeInfo{
			Fingerprint:     s.Fingerprint,
			SenderDomain:    s.SenderDomain,
			SubjectTemplate: s.SubjectTemplate,
			LastSlot:        s.LastSlot,
			Status:          s.Status,
			HitCount:        s.HitCount,
			ConfirmCount:    s.ConfirmCount,
			RejectCount:     s.RejectCount,
		})
		ph := strings.TrimSpace(s.PromptHash)
		if ph != "" {
			promptHashes[ph]++
		}
	}
	errOut := make([]ErrorRowInfo, 0, len(errRows))
	for _, e := range errRows {
		errOut = append(errOut, ErrorRowInfo{
			ObjectHash:   e.ObjectHash,
			Error:        e.Error,
			AttemptCount: e.AttemptCount,
			UpdatedAt:    e.UpdatedAt,
		})
	}
	matched := 0
	for _, it := range items {
		if it.hasMail {
			matched++
		}
	}

	eg := errorGroupsFromRows(errRows)

	m := Model{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   now.UTC().Format(time.RFC3339),
		LedgerPath:    ledgerPath,
		MailStorePath: mailStorePath,
		CLIVersion:    version,
		Provenance: Provenance{
			CompleteRows: len(items),
			MailMatched:  matched,
			MailMissing:  len(items) - matched,
			GitRevision:  gitRevision,
		},
		Months:    sections,
		All:       allSec,
		Shapes:    shapeOut,
		ErrorRows: errOut,
	}
	// Global prompt hash rollups for the pipeline section on "All"
	m.All.Pipeline.PromptHashes = promptHashes
	// Attach error rollups to All trust section for visibility
	m.All.Trust.ErrorGroups = eg
	return m
}

func errorGroupsFromRows(errRows []hoststore.ErrorRow) []ErrorGroup {
	type agg struct {
		rows    int
		attempt int
	}
	m := map[string]*agg{}
	for _, e := range errRows {
		k := strings.TrimSpace(e.Error)
		if k == "" {
			k = "(empty error)"
		}
		if m[k] == nil {
			m[k] = &agg{}
		}
		m[k].rows++
		ac := e.AttemptCount
		if ac < 1 {
			ac = 1
		}
		m[k].attempt += ac
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]ErrorGroup, 0, len(keys))
	for _, k := range keys {
		a := m[k]
		out = append(out, ErrorGroup{Error: k, Rows: a.rows, TotalAttempts: a.attempt})
	}
	return out
}

func inRangeMonth(m, since, until string) bool {
	if m == "unknown" {
		return since == "" && until == ""
	}
	if since != "" && m < since {
		return false
	}
	if until != "" && m > until {
		return false
	}
	return true
}

func filterMonth(items []enriched, month string) []enriched {
	var out []enriched
	for _, it := range items {
		if it.month == month {
			out = append(out, it)
		}
	}
	return out
}

func monthLabel(monthKey string) string {
	if monthKey == "unknown" {
		return "Unknown month"
	}
	if monthKey == "all" {
		return "All"
	}
	if len(monthKey) >= 7 {
		if t, err := time.ParseInLocation("2006-01", monthKey[:7], time.Local); err == nil {
			return t.Format("Jan 2006")
		}
	}
	return monthKey
}

func lastNMonthKeys(now time.Time, loc *time.Location, n int) []string {
	if loc == nil {
		loc = time.Local
	}
	t := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	var rev []string
	for i := 0; i < n; i++ {
		rev = append(rev, t.Format("2006-01"))
		t = t.AddDate(0, -1, 0)
	}
	// oldest first
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

func buildMonthSection(
	key, label string,
	items []enriched,
	prevMonthKey string,
	sparkMonths []string,
	seriesByMonth map[string]int,
	in Input,
) MonthSection {
	sec := MonthSection{Key: key, Label: label}
	sec.Summary = summaryFromItems(items, key, prevMonthKey, sparkMonths, seriesByMonth)
	sec.Attention = attentionFromItems(items)
	sec.Themes, sec.Heatmap = themesAndHeatmap(items)
	sec.Fluff = fluffFromItems(items)
	kt := kindTreeFromItems(items, key)
	sec.KindTree = kt
	sec.Pipeline = pipelineFromItems(items)
	if key == "all" {
		// PromptHashes set on Model after build
	} else {
		sec.Pipeline.PromptHashes = map[string]int{}
	}
	sec.Trust = trustFromItems(items)
	sec.Messages = messageRows(items, key, kt)
	return sec
}

// CapMessageTables limits Messages on each month and All (for HTML). JSON should marshal before calling this.
func CapMessageTables(m *Model, max int) {
	if max <= 0 {
		return
	}
	capSec := func(sec *MonthSection) {
		if len(sec.Messages) > max {
			sec.Messages = sec.Messages[:max]
		}
	}
	for i := range m.Months {
		capSec(&m.Months[i])
	}
	capSec(&m.All)
}

func prevCalendarMonthKey(m string) string {
	if len(m) < 7 {
		return ""
	}
	t, err := time.ParseInLocation("2006-01", m[:7], time.Local)
	if err != nil {
		return ""
	}
	return t.AddDate(0, -1, 0).Format("2006-01")
}

func summaryFromItems(
	items []enriched,
	monthKey string,
	prevMonthKey string,
	sparkMonths []string,
	seriesByMonth map[string]int,
) SummaryStats {
	var s SummaryStats
	skipStrat := mailreport.StrategyShapeConfirm
	for _, it := range items {
		s.Classified++
		if strings.TrimSpace(it.row.Strategy) == skipStrat {
			s.JevSkips++
		} else {
			s.EssenceChats++
		}
		if it.row.NeedsReplyNoul >= urgThreshold {
			s.NeedsReply++
		}
	}
	s.EmbedsSaved = s.JevSkips * hopsPerSkip
	for _, it := range items {
		s.FluffMinutes += float64(deleteFluffSecs) / 60
		if it.row.NeedsReplyNoul >= urgThreshold || it.row.InvolvesDecisionNoul >= urgThreshold {
			s.FluffMinutes += float64(keepFluffSecs-deleteFluffSecs) / 60
		}
	}
	for _, m := range sparkMonths {
		s.SparkMonths = append(s.SparkMonths, m)
		s.Sparkline = append(s.Sparkline, seriesByMonth[m])
	}
	if monthKey != "all" && monthKey != "unknown" && monthKey != "" {
		cur := len(items)
		prev := 0
		if prevMonthKey != "" {
			prev = seriesByMonth[prevMonthKey]
		}
		s.DeltaClassified = cur - prev
	}
	return s
}

func attentionFromItems(items []enriched) AttentionBlock {
	var a AttentionBlock
	a.UrgencyHist = Histogram{Buckets: linspace(0, 1, 11), Counts: make([]int, 10), Threshold: urgThreshold}
	for i := range a.UrgencyHist.Counts {
		lo := a.UrgencyHist.Buckets[i]
		hi := a.UrgencyHist.Buckets[i+1]
		if i == len(a.UrgencyHist.Counts)-1 {
			hi = math.Nextafter(hi, hi+1)
		}
		for _, it := range items {
			u := it.row.Urgency
			if u >= lo && u < hi {
				a.UrgencyHist.Counts[i]++
			}
		}
	}
	for _, it := range items {
		rep := it.row.NeedsReplyNoul >= urgThreshold
		dec := it.row.InvolvesDecisionNoul >= urgThreshold
		switch {
		case rep:
			a.NeedsReply++
			a.ReplyList = append(a.ReplyList, messageLineFrom(it))
		case dec:
			a.Decision++
			a.DecisionList = append(a.DecisionList, messageLineFrom(it))
		case it.row.Urgency >= urgThreshold:
			a.UrgentOnly++
		default:
			a.Quiet++
		}
	}
	total := len(items)
	if total == 0 {
		total = 1
	}
	a.Donut = []DonutSlice{
		{Label: "Quiet", Count: a.Quiet, Fraction: float64(a.Quiet) / float64(total)},
		{Label: "Urgent only", Count: a.UrgentOnly, Fraction: float64(a.UrgentOnly) / float64(total)},
		{Label: "Needs reply", Count: a.NeedsReply, Fraction: float64(a.NeedsReply) / float64(total)},
		{Label: "Decision", Count: a.Decision, Fraction: float64(a.Decision) / float64(total)},
	}
	return a
}

func messageLineFrom(it enriched) MessageLine {
	return MessageLine{
		ObjectHash: it.row.ObjectHash,
		Date:       it.summary.Date,
		Sender:     it.email,
		SenderName: it.name,
		Subject:    it.summary.Subject,
		Theme:      it.theme,
		Kind:       it.row.Kind,
		Urgency:    it.row.Urgency,
		NeedsReply: it.row.NeedsReplyNoul,
		Decision:   it.row.InvolvesDecisionNoul,
		Cosine:     it.row.Cosine,
		JudgeScore: it.row.JudgeScore,
	}
}

func themesAndHeatmap(items []enriched) ([]ThemeCount, HeatmapData) {
	counts := map[string]int{}
	for _, it := range items {
		counts[it.theme]++
	}
	// "Other" is never ranked: it always goes last, holding unnamed themes
	// plus any named theme beyond the top eight, so it appears only once.
	var themeNames []string
	for k := range counts {
		if k != themeOther {
			themeNames = append(themeNames, k)
		}
	}
	sort.Slice(themeNames, func(i, j int) bool {
		if counts[themeNames[i]] != counts[themeNames[j]] {
			return counts[themeNames[i]] > counts[themeNames[j]]
		}
		return themeNames[i] < themeNames[j]
	})
	top := 8
	if len(themeNames) < top {
		top = len(themeNames)
	}
	tc := make([]ThemeCount, 0, top+1)
	heatThemes := make([]string, 0, top+1)
	for _, th := range themeNames[:top] {
		tc = append(tc, ThemeCount{Slot: th, Count: counts[th]})
		heatThemes = append(heatThemes, th)
	}
	otherSum := counts[themeOther]
	for _, th := range themeNames[top:] {
		otherSum += counts[th]
	}
	if otherSum > 0 {
		tc = append(tc, ThemeCount{Slot: themeOther, Count: otherSum})
	}
	heatThemes = append(heatThemes, themeOther)

	weekSet := map[string]struct{}{}
	for _, it := range items {
		if it.weekKey != "" {
			weekSet[it.weekKey] = struct{}{}
		}
	}
	var weeks []string
	for w := range weekSet {
		weeks = append(weeks, w)
	}
	sort.Strings(weeks)

	cells := make([][]int, len(weeks))
	cellHashLists := make([][][]string, len(weeks))
	widx := map[string]int{}
	for wi, w := range weeks {
		widx[w] = wi
		cells[wi] = make([]int, len(heatThemes))
		cellHashLists[wi] = make([][]string, len(heatThemes))
	}
	tidx := map[string]int{}
	for i, t := range heatThemes {
		tidx[t] = i
	}
	themeHashLists := make([][]string, len(heatThemes))
	for _, it := range items {
		if it.weekKey == "" {
			continue
		}
		wi, ok := widx[it.weekKey]
		if !ok {
			continue
		}
		slot := it.theme
		if _, ok := tidx[slot]; !ok {
			slot = themeOther
		}
		ti := tidx[slot]
		cells[wi][ti]++
		if h := strings.TrimSpace(it.row.ObjectHash); h != "" {
			cellHashLists[wi][ti] = append(cellHashLists[wi][ti], h)
			themeHashLists[ti] = append(themeHashLists[ti], h)
		}
	}
	cellHashes := make([][]string, len(weeks))
	for wi := range weeks {
		cellHashes[wi] = make([]string, len(heatThemes))
		for ti := range heatThemes {
			cellHashes[wi][ti] = JoinHashes(cellHashLists[wi][ti])
		}
	}
	themeHashes := make([]string, len(heatThemes))
	for ti := range heatThemes {
		themeHashes[ti] = JoinHashes(themeHashLists[ti])
	}
	return tc, HeatmapData{
		WeekLabels:  weeks,
		ThemeLabels: heatThemes,
		Cells:       cells,
		CellHashes:  cellHashes,
		ThemeHashes: themeHashes,
	}
}

func kindTreeFromItems(items []enriched, viewKey string) KindTreeBlock {
	firstCount := map[string]int{}
	pathCount := map[string]int{}
	pathHashes := map[string][]string{}
	for _, it := range items {
		f := firstKindSegment(it.row.Kind)
		if f == "" {
			f = "(empty)"
		}
		firstCount[f]++
		p := strings.TrimSpace(it.row.Kind)
		if p == "" {
			p = "(empty)"
		}
		pathCount[p]++
		if h := strings.TrimSpace(it.row.ObjectHash); h != "" {
			pathHashes[p] = append(pathHashes[p], h)
		}
	}
	var trunk []KindTrunkNode
	var outliers []KindOutlierLeaf
	var firstSlots []string
	for s := range firstCount {
		firstSlots = append(firstSlots, s)
	}
	sort.Strings(firstSlots)
	for _, slot := range firstSlots {
		c := firstCount[slot]
		if c <= 1 {
			continue
		}
		trunk = append(trunk, KindTrunkNode{FirstSlot: slot, Count: c})
	}
	sort.Slice(trunk, func(i, j int) bool {
		if trunk[i].Count != trunk[j].Count {
			return trunk[i].Count > trunk[j].Count
		}
		return trunk[i].FirstSlot < trunk[j].FirstSlot
	})
	for _, slot := range firstSlots {
		if firstCount[slot] != 1 {
			continue
		}
		var kindPath string
		for _, it := range items {
			f := firstKindSegment(it.row.Kind)
			if f == "" {
				f = "(empty)"
			}
			if f == slot {
				kindPath = strings.TrimSpace(it.row.Kind)
				break
			}
		}
		if kindPath == "" {
			kindPath = slot
		}
		why := fmt.Sprintf(`Only mail in view %q whose first slot is %q. KIND is %s.`, viewKey, slot, kindPath)
		outliers = append(outliers, KindOutlierLeaf{FirstSlot: slot, Kind: kindPath, Why: why})
	}
	sort.Slice(outliers, func(i, j int) bool { return outliers[i].FirstSlot < outliers[j].FirstSlot })

	var paths []string
	for p := range pathCount {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var pathRows []KindPathRow
	for _, p := range paths {
		f := firstKindSegment(p)
		if f == "" {
			f = "(empty)"
		}
		why := ""
		if firstCount[f] == 1 {
			why = fmt.Sprintf(`Only mail in view %q whose first slot is %q. KIND is %s.`, viewKey, f, p)
		}
		pathRows = append(pathRows, KindPathRow{Path: p, Count: pathCount[p], Why: why, Hashes: pathHashes[p]})
	}
	sort.Slice(pathRows, func(i, j int) bool {
		if pathRows[i].Count != pathRows[j].Count {
			return pathRows[i].Count > pathRows[j].Count
		}
		return pathRows[i].Path < pathRows[j].Path
	})
	return KindTreeBlock{Trunk: trunk, Outliers: outliers, Paths: pathRows}
}

func fluffFromItems(items []enriched) FluffBlock {
	byWeek := map[string]float64{}
	sender := map[string]float64{}
	senderCount := map[string]int{}
	senderHashes := map[string][]string{}
	var deleteMin, keepExtra float64
	for _, it := range items {
		wk := it.weekKey
		if wk == "" {
			wk = "(no week)"
		}
		del := float64(deleteFluffSecs) / 60
		byWeek[wk] += del
		sender[it.email] += del
		senderCount[it.email]++
		if h := strings.TrimSpace(it.row.ObjectHash); h != "" {
			senderHashes[it.email] = append(senderHashes[it.email], h)
		}
		deleteMin += del
		if it.row.NeedsReplyNoul >= urgThreshold || it.row.InvolvesDecisionNoul >= urgThreshold {
			ex := float64(keepFluffSecs-deleteFluffSecs) / 60
			byWeek[wk] += ex
			sender[it.email] += ex
			keepExtra += ex
		}
	}
	var wkList []WeekFluff
	for k, v := range byWeek {
		wkList = append(wkList, WeekFluff{WeekKey: k, Label: k, Minutes: v})
	}
	sort.Slice(wkList, func(i, j int) bool { return wkList[i].WeekKey < wkList[j].WeekKey })
	var sendList []SenderFluff
	for k, v := range sender {
		sendList = append(sendList, SenderFluff{Sender: k, Minutes: v, Messages: senderCount[k], Hashes: senderHashes[k]})
	}
	sort.Slice(sendList, func(i, j int) bool { return sendList[i].Minutes > sendList[j].Minutes })
	if len(sendList) > 10 {
		sendList = sendList[:10]
	}
	return FluffBlock{
		ByWeek:        wkList,
		TopSenders:    sendList,
		DeleteMinutes: deleteMin,
		KeepMinutes:   keepExtra,
	}
}

func pipelineFromItems(items []enriched) PipelineBlock {
	var p PipelineBlock
	p.ModelsClassify = map[string]int{}
	p.ModelsEmbed = map[string]int{}
	p.PromptHashes = map[string]int{}
	p.HopFlow = HopFlow{
		Nodes: []string{"Messages", "Jev_skip", "Essence", "Classified"},
	}
	skipStrat := mailreport.StrategyShapeConfirm
	jev := 0
	essence := 0
	weekSkip := map[string]struct{ skips, total int }{}
	for _, it := range items {
		cm := strings.TrimSpace(it.row.ClassifyModel)
		if cm != "" {
			p.ModelsClassify[cm]++
		}
		em := strings.TrimSpace(it.row.EmbedModel)
		if em != "" {
			p.ModelsEmbed[em]++
		}
		wk := it.weekKey
		if wk == "" {
			wk = "(no week)"
		}
		w := weekSkip[wk]
		w.total++
		if strings.TrimSpace(it.row.Strategy) == skipStrat {
			jev++
			w.skips++
		} else {
			essence++
		}
		weekSkip[wk] = w
	}
	p.HopFlow.Edges = []HopEdge{
		{From: "Messages", To: "Jev_skip", Count: jev},
		{From: "Messages", To: "Essence", Count: essence},
		{From: "Jev_skip", To: "Classified", Count: jev},
		{From: "Essence", To: "Classified", Count: essence},
	}
	var wks []string
	for k := range weekSkip {
		wks = append(wks, k)
	}
	sort.Strings(wks)
	for _, wk := range wks {
		v := weekSkip[wk]
		rate := 0.0
		if v.total > 0 {
			rate = float64(v.skips) / float64(v.total)
		}
		p.SkipRateWeek = append(p.SkipRateWeek, WeekPoint{
			WeekKey: wk,
			Label:   wk,
			Rate:    rate,
			Skips:   v.skips,
			Total:   v.total,
		})
	}
	return p
}

func trustFromItems(items []enriched) TrustBlock {
	var t TrustBlock
	t.CosineHist = Histogram{Buckets: linspace(0, 1, 11), Counts: make([]int, 10), Threshold: 0}
	t.JudgeHist = Histogram{Buckets: linspace(0, 1, 11), Counts: make([]int, 10), Threshold: 0}
	type scored struct {
		it    enriched
		score float64
	}
	var low []scored
	for _, it := range items {
		for i := range t.CosineHist.Counts {
			lo := t.CosineHist.Buckets[i]
			hi := t.CosineHist.Buckets[i+1]
			if i == len(t.CosineHist.Counts)-1 {
				hi = math.Nextafter(hi, hi+1)
			}
			c := it.row.Cosine
			if c >= lo && c < hi {
				t.CosineHist.Counts[i]++
			}
		}
		for i := range t.JudgeHist.Counts {
			lo := t.JudgeHist.Buckets[i]
			hi := t.JudgeHist.Buckets[i+1]
			if i == len(t.JudgeHist.Counts)-1 {
				hi = math.Nextafter(hi, hi+1)
			}
			j := it.row.JudgeScore
			if j >= lo && j < hi {
				t.JudgeHist.Counts[i]++
			}
		}
		if !it.row.AttentionScored {
			t.NotAttention++
		}
		low = append(low, scored{it: it, score: it.row.Cosine})
	}
	sort.Slice(low, func(i, j int) bool {
		if low[i].score != low[j].score {
			return low[i].score < low[j].score
		}
		return low[i].it.row.ObjectHash < low[j].it.row.ObjectHash
	})
	for i := 0; i < len(low) && i < 20; i++ {
		t.LowConf = append(t.LowConf, messageLineFrom(low[i].it))
	}
	return t
}

func messageRows(items []enriched, viewKey string, kt KindTreeBlock) []MessageRow {
	firstCount := map[string]int{}
	for _, n := range kt.Trunk {
		firstCount[n.FirstSlot] += n.Count
	}
	for _, o := range kt.Outliers {
		firstCount[o.FirstSlot] = 1
	}
	out := make([]MessageRow, 0, len(items))
	for _, it := range items {
		f := firstKindSegment(it.row.Kind)
		if f == "" {
			f = "(empty)"
		}
		kindPath := strings.TrimSpace(it.row.Kind)
		if kindPath == "" {
			kindPath = "(empty)"
		}
		why := ""
		if firstCount[f] == 1 {
			why = fmt.Sprintf(`Only mail in view %q whose first slot is %q. KIND is %s.`, viewKey, f, kindPath)
		}
		out = append(out, MessageRow{
			MessageLine:  messageLineFrom(it),
			Strategy:     it.row.Strategy,
			WeekKey:      it.weekKey,
			MonthKey:     it.month,
			OutlierFirst: firstCount[f] == 1,
			Why:          why,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].ObjectHash < out[j].ObjectHash
	})
	return out
}

func linspace(lo, hi float64, n int) []float64 {
	if n < 2 {
		return []float64{lo, hi}
	}
	var out []float64
	for i := 0; i < n; i++ {
		out = append(out, lo+(hi-lo)*float64(i)/float64(n-1))
	}
	return out
}
