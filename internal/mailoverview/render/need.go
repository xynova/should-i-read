package render

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xynova/should-i-read/internal/mailoverview"
)

// visibleActionRows is how many grouped rows a list shows before "Show more".
const visibleActionRows = 8

// deleteMinutesPerMessage matches aggregate delete fluff seconds as minutes.
const deleteMinutesPerMessage = 5.0 / 60.0

// AttentionTile is one count tile at the top of "What needs you".
type AttentionTile struct {
	Label string
	Hint  string
	Count int
	// Tone picks the tile colour: reply, decide, urgent or safe.
	Tone string
}

// ActionRow is one sender and subject, grouped across repeats.
type ActionRow struct {
	Date     string
	FullDate string
	Name     string
	// Address is the sender email, empty when Name already shows it.
	Address string
	Subject string
	// TLDR is the classified About sentence for this grouped row.
	TLDR  string
	Count int
	Score float64
	// Hashes are object_hash values in this grouped row.
	Hashes []string
	// HashAttr is Hashes joined for data-hashes.
	HashAttr string
	// Extra marks rows past visibleActionRows, hidden until "Show more".
	Extra bool
}

// ActionList is the Reply or Decide list for one tab.
type ActionList struct {
	Title string
	Tone  string
	// Messages counts every message in the list, before grouping.
	Messages   int
	Rows       []ActionRow
	ExtraCount int
}

// DeleteSenderRow is one quiet sender plus expandable message entries.
type DeleteSenderRow struct {
	Name     string
	Address  string
	Messages int
	Minutes  float64
	HashAttr string
	ExpandID string
	Extra    bool
	Entries  []ActionRow
}

// DeleteListView is the Delete list under What needs you.
type DeleteListView struct {
	Title      string
	Messages   int
	Senders    []DeleteSenderRow
	ExtraCount int
}

func attentionTiles(a mailoverview.AttentionBlock) []AttentionTile {
	return []AttentionTile{
		{Label: "Need a reply", Hint: "Someone is waiting on you", Count: a.NeedsReply, Tone: "reply"},
		{Label: "Need a decision", Hint: "Approve, pay, accept or act", Count: a.Decision, Tone: "decide"},
		{Label: "Urgent, no action", Hint: "Time-sensitive, read only", Count: a.UrgentOnly, Tone: "urgent"},
		{Label: "Can wait", Hint: "Safe to skim or delete", Count: a.Quiet, Tone: "safe"},
	}
}

// actionLists returns the Reply list, then Decide, then Urgent, each grouped
// by sender and subject. Dates in refYear are shown without the year.
func actionLists(a mailoverview.AttentionBlock, refYear int) []ActionList {
	var out []ActionList
	if len(a.ReplyList) > 0 {
		out = append(out, groupActions("Reply", "reply", a.ReplyList, refYear, func(m mailoverview.MessageLine) float64 { return m.NeedsReply }))
	}
	if len(a.DecisionList) > 0 {
		out = append(out, groupActions("Decide", "decide", a.DecisionList, refYear, func(m mailoverview.MessageLine) float64 { return m.Decision }))
	}
	if len(a.UrgentList) > 0 {
		out = append(out, groupActions("Urgent", "urgent", a.UrgentList, refYear, func(m mailoverview.MessageLine) float64 { return m.Urgency }))
	}
	return out
}

func groupActions(title, tone string, lines []mailoverview.MessageLine, refYear int, score func(mailoverview.MessageLine) float64) ActionList {
	type group struct {
		row    ActionRow
		latest string
	}
	byKey := map[string]*group{}
	var order []string
	for _, m := range lines {
		key := strings.ToLower(strings.TrimSpace(m.Sender)) + "\x00" + subjectKey(m.Subject)
		g, ok := byKey[key]
		if !ok {
			name, addr := senderParts(m)
			g = &group{row: ActionRow{Name: name, Address: addr}}
			byKey[key] = g
			order = append(order, key)
		}
		g.row.Count++
		if h := strings.TrimSpace(m.ObjectHash); h != "" {
			g.row.Hashes = append(g.row.Hashes, h)
		}
		if s := score(m); s > g.row.Score {
			g.row.Score = s
		}
		if m.Date >= g.latest {
			g.latest = m.Date
			g.row.Subject = strings.TrimSpace(m.Subject)
			g.row.TLDR = strings.TrimSpace(m.About)
			g.row.Date = shortDate(m.Date, refYear)
			g.row.FullDate = isoDate(m.Date)
		}
	}
	rows := make([]ActionRow, 0, len(order))
	for _, k := range order {
		g := byKey[k]
		g.row.HashAttr = strings.Join(g.row.Hashes, ",")
		rows = append(rows, g.row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].FullDate > rows[j].FullDate
	})
	extra := 0
	for i := range rows {
		if i >= visibleActionRows {
			rows[i].Extra = true
			extra++
		}
	}
	return ActionList{Title: title, Tone: tone, Messages: len(lines), Rows: rows, ExtraCount: extra}
}

func groupDeleteSenders(lines []mailoverview.MessageLine, refYear int, tabKey string) DeleteListView {
	if len(lines) == 0 {
		return DeleteListView{}
	}
	type group struct {
		row     DeleteSenderRow
		hashes  []string
		entries []ActionRow
	}
	byKey := map[string]*group{}
	var order []string
	for _, m := range lines {
		key := deleteGroupKey(m)
		g, ok := byKey[key]
		name, addr := senderParts(m)
		if name == "" {
			name = key
		}
		if !ok {
			g = &group{row: DeleteSenderRow{Name: name, Address: addr}}
			byKey[key] = g
			order = append(order, key)
		}
		h := strings.TrimSpace(m.ObjectHash)
		var hashes []string
		if h != "" {
			hashes = []string{h}
			g.hashes = append(g.hashes, h)
		}
		g.entries = append(g.entries, ActionRow{
			Name:     g.row.Name,
			Address:  g.row.Address,
			Subject:  strings.TrimSpace(m.Subject),
			Date:     shortDate(m.Date, refYear),
			FullDate: isoDate(m.Date),
			Count:    1,
			Hashes:   hashes,
			HashAttr: strings.Join(hashes, ","),
		})
	}
	senders := make([]DeleteSenderRow, 0, len(order))
	for _, k := range order {
		g := byKey[k]
		sort.SliceStable(g.entries, func(i, j int) bool {
			return g.entries[i].FullDate > g.entries[j].FullDate
		})
		g.row.Entries = g.entries
		g.row.Messages = len(g.entries)
		g.row.Minutes = float64(g.row.Messages) * deleteMinutesPerMessage
		g.row.HashAttr = mailoverview.JoinHashes(g.hashes)
		senders = append(senders, g.row)
	}
	sort.SliceStable(senders, func(i, j int) bool {
		if senders[i].Messages != senders[j].Messages {
			return senders[i].Messages > senders[j].Messages
		}
		return senders[i].Name < senders[j].Name
	})
	prefix := strings.TrimSpace(tabKey)
	if prefix == "" {
		prefix = "tab"
	}
	extra := 0
	for i := range senders {
		senders[i].ExpandID = fmt.Sprintf("del-%s-%d", prefix, i)
		if i >= visibleActionRows {
			senders[i].Extra = true
			extra++
		}
	}
	return DeleteListView{Title: "Delete", Messages: len(lines), Senders: senders, ExtraCount: extra}
}

func deleteGroupKey(m mailoverview.MessageLine) string {
	addr := strings.ToLower(strings.TrimSpace(m.Sender))
	if addr == "" {
		addr = "(unknown sender)"
	}
	name := strings.ToLower(strings.TrimSpace(m.SenderName))
	if name == "" || name == addr {
		return addr
	}
	return name + "\x00" + addr
}

// senderParts returns the display name and, when it differs, the address.
func senderParts(m mailoverview.MessageLine) (string, string) {
	addr := strings.TrimSpace(m.Sender)
	name := strings.TrimSpace(m.SenderName)
	if name == "" || strings.EqualFold(name, addr) {
		return addr, ""
	}
	return name, addr
}

// subjectKey folds case, punctuation and emoji so repeats group together.
func subjectKey(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

func isoDate(s string) string {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}

// shortDate turns "2026-10-07..." into "7 Oct", adding the year when it is
// not refYear.
func shortDate(s string, refYear int) string {
	t, err := time.Parse("2006-01-02", isoDate(s))
	if err != nil {
		return s
	}
	if t.Year() == refYear {
		return t.Format("2 Jan")
	}
	return t.Format("2 Jan 2006")
}

// weekLabel turns a Monday week key "2026-09-28" into "28 Sep".
func weekLabel(key string) string {
	if key == "" || key == mailoverview.NoWeekKey {
		return "No date"
	}
	t, err := time.Parse("2006-01-02", key)
	if err != nil {
		return key
	}
	return t.Format("2 Jan")
}

// labelAcronyms are KIND words that read as upper-case acronyms.
var labelAcronyms = map[string]string{
	"ci": "CI", "pr": "PR", "api": "API", "ai": "AI", "hr": "HR", "it": "IT", "sso": "SSO", "2fa": "2FA",
}

// humanLabel turns a KIND slot such as "workflow-failure" into "Workflow failure".
func humanLabel(slot string) string {
	s := strings.TrimSpace(strings.ReplaceAll(slot, "-", " "))
	if s == "" {
		return slot
	}
	words := strings.Fields(s)
	for i, w := range words {
		if a, ok := labelAcronyms[strings.ToLower(w)]; ok {
			words[i] = a
		}
	}
	s = strings.Join(words, " ")
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// generatedLabel formats the RFC 3339 stamp as "Fri 6 22:50" (weekday, day, UTC time).
func generatedLabel(stamp string) string {
	t, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return stamp
	}
	return t.UTC().Format("Mon 2 15:04")
}

// yearOf returns the year of an RFC 3339 stamp, or 0 when it does not parse.
func yearOf(stamp string) int {
	if len(stamp) < 4 {
		return 0
	}
	y, err := strconv.Atoi(stamp[:4])
	if err != nil {
		return 0
	}
	return y
}

// isMinorTab reports whether a month tab belongs in the Previous pulldown.
func isMinorTab(key, generatedAt string) bool {
	if key == "unknown" {
		return true
	}
	return !isRecentMonthKey(key, generatedAt)
}

// isRecentMonthKey reports whether key is in the last recentMonthCount
// calendar months of generatedAt, including that month.
func isRecentMonthKey(key, generatedAt string) bool {
	now, err := time.Parse(time.RFC3339, generatedAt)
	if err != nil {
		return false
	}
	month, err := time.Parse("2006-01", key)
	if err != nil {
		return false
	}
	now = now.UTC()
	end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 1-recentMonthCount, 0)
	m := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	return !m.Before(start) && !m.After(end)
}

// defaultTabKey opens the latest main month with mail, else the latest previous
// month with mail, else All. The unknown month is never opened by default.
func defaultTabKey(views []TabView) string {
	main, stray := "", ""
	for _, v := range views {
		if v.Key == "all" || v.Key == "unknown" || v.Section.Summary.Classified == 0 {
			continue
		}
		if v.Minor {
			stray = v.Key
		} else {
			main = v.Key
		}
	}
	switch {
	case main != "":
		return main
	case stray != "":
		return stray
	default:
		return "all"
	}
}
