package mailoverview

import (
	"sort"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
)

// ActionedRow is one concise sender group on the actioned strip.
type ActionedRow struct {
	Sender   string   `json:"sender"`
	Count    int      `json:"count"`
	From     string   `json:"from,omitempty"`
	Until    string   `json:"until,omitempty"`
	Outcome  string   `json:"outcome"`
	MonthKey string   `json:"month_key"`
	Hashes   []string `json:"hashes"`
}

// HashAttr joins hashes for data-hashes.
func (r ActionedRow) HashAttr() string {
	return JoinHashes(r.Hashes)
}

// WhenLabel is the concise date range for the actioned strip.
func (r ActionedRow) WhenLabel() string {
	if strings.TrimSpace(r.From) == "" {
		return ""
	}
	if r.Until == "" || r.Until == r.From {
		return r.From
	}
	return r.From + " through " + r.Until
}

func shortActionedDate(raw string) string {
	t, ok := parseMailDate(raw, time.UTC)
	if !ok {
		return strings.TrimSpace(raw)
	}
	return t.Format("2 Jan 2006")
}

func actionedMonthKey(raw string) string {
	t, ok := parseMailDate(raw, time.UTC)
	if !ok {
		return monthKeyUnknown
	}
	return t.Format("2006-01")
}

// DropActionedRows keeps overview rows that are still live on the month board.
func DropActionedRows(rows []hoststore.OverviewRow, actioned map[string]maildelete.ActionedInfo) []hoststore.OverviewRow {
	if len(actioned) == 0 {
		return rows
	}
	out := make([]hoststore.OverviewRow, 0, len(rows))
	for _, r := range rows {
		if _, ok := actioned[strings.TrimSpace(r.ObjectHash)]; ok {
			continue
		}
		out = append(out, r)
	}
	return out
}

func dropActionedItems(items []enriched, actioned map[string]maildelete.ActionedInfo) []enriched {
	if len(actioned) == 0 {
		return items
	}
	out := make([]enriched, 0, len(items))
	for _, it := range items {
		if _, ok := actioned[strings.TrimSpace(it.row.ObjectHash)]; ok {
			continue
		}
		out = append(out, it)
	}
	return out
}

// GroupActioned rolls actioned hashes into sender rows by month. It is not a kind tree.
func GroupActioned(actioned map[string]maildelete.ActionedInfo) []ActionedRow {
	type agg struct {
		sender   string
		outcome  string
		monthKey string
		hashes   []string
		dates    []string
	}
	groups := map[string]*agg{}
	for h, info := range actioned {
		if strings.TrimSpace(info.Outcome) == "" {
			continue
		}
		sender := strings.TrimSpace(info.Sender)
		if sender == "" {
			sender = "(unknown sender)"
		}
		monthKey := actionedMonthKey(info.Date)
		key := sender + "\x00" + info.Outcome + "\x00" + monthKey
		g, ok := groups[key]
		if !ok {
			g = &agg{sender: sender, outcome: info.Outcome, monthKey: monthKey}
			groups[key] = g
		}
		g.hashes = append(g.hashes, h)
		if d := strings.TrimSpace(info.Date); d != "" {
			g.dates = append(g.dates, d)
		}
	}
	out := make([]ActionedRow, 0, len(groups))
	for _, g := range groups {
		sort.Strings(g.hashes)
		sort.Strings(g.dates)
		row := ActionedRow{
			Sender:   g.sender,
			Count:    len(g.hashes),
			Outcome:  g.outcome,
			MonthKey: g.monthKey,
			Hashes:   g.hashes,
		}
		if len(g.dates) > 0 {
			row.From = shortActionedDate(g.dates[0])
			row.Until = shortActionedDate(g.dates[len(g.dates)-1])
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sender != out[j].Sender {
			return out[i].Sender < out[j].Sender
		}
		if out[i].MonthKey != out[j].MonthKey {
			return out[i].MonthKey < out[j].MonthKey
		}
		return out[i].Outcome < out[j].Outcome
	})
	return out
}

// ActionedCountForMonth is how many actioned messages sit in key.
// key "all" (or empty) is the whole board.
func ActionedCountForMonth(rows []ActionedRow, key string) int {
	n := 0
	for _, r := range rows {
		if key == "" || key == monthKeyAll || r.MonthKey == key {
			n += r.Count
		}
	}
	return n
}

const actionedSenderBarLimit = 8

// NamedCount is one bar on the Report Actioned charts.
type NamedCount struct {
	Label string
	Count int
}

// ActionedOutcomeCounts rolls Actioned rows into outcome bars.
func ActionedOutcomeCounts(rows []ActionedRow) []NamedCount {
	totals := map[string]int{}
	for _, row := range rows {
		label := strings.TrimSpace(row.Outcome)
		if label == "" || row.Count < 1 {
			continue
		}
		totals[label] += row.Count
	}
	order := []string{
		maildelete.OutcomeTrashedAtSource,
		maildelete.OutcomeAlreadyInTrash,
		maildelete.OutcomeAckedAtSource,
	}
	out := make([]NamedCount, 0, len(totals))
	seen := map[string]struct{}{}
	for _, label := range order {
		if n := totals[label]; n > 0 {
			out = append(out, NamedCount{Label: label, Count: n})
			seen[label] = struct{}{}
		}
	}
	extra := make([]string, 0)
	for label := range totals {
		if _, ok := seen[label]; !ok {
			extra = append(extra, label)
		}
	}
	sort.Strings(extra)
	for _, label := range extra {
		out = append(out, NamedCount{Label: label, Count: totals[label]})
	}
	return out
}

// ActionedSenderCounts rolls Actioned rows into the loudest senders.
func ActionedSenderCounts(rows []ActionedRow, limit int) []NamedCount {
	if limit < 1 {
		limit = actionedSenderBarLimit
	}
	totals := map[string]int{}
	for _, row := range rows {
		sender := strings.TrimSpace(row.Sender)
		if sender == "" || row.Count < 1 {
			continue
		}
		totals[sender] += row.Count
	}
	out := make([]NamedCount, 0, len(totals))
	for sender, n := range totals {
		out = append(out, NamedCount{Label: sender, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func filterCartAdds(existing []hoststore.DeleteCartItem, incoming []hoststore.DeleteCartItem, actioned map[string]maildelete.ActionedInfo) (toAdd []hoststore.DeleteCartItem, skippedInCart, skippedActioned int) {
	inCart := map[string]struct{}{}
	for _, item := range existing {
		h := strings.TrimSpace(item.ObjectHash)
		if h != "" {
			inCart[h] = struct{}{}
		}
	}
	if actioned == nil {
		actioned = map[string]maildelete.ActionedInfo{}
	}
	seen := map[string]struct{}{}
	for _, item := range incoming {
		h := strings.TrimSpace(item.ObjectHash)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		if _, ok := inCart[h]; ok {
			skippedInCart++
			continue
		}
		if _, ok := actioned[h]; ok {
			skippedActioned++
			continue
		}
		toAdd = append(toAdd, item)
	}
	return toAdd, skippedInCart, skippedActioned
}
