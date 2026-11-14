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
	Sender  string   `json:"sender"`
	Count   int      `json:"count"`
	From    string   `json:"from,omitempty"`
	Until   string   `json:"until,omitempty"`
	Outcome string   `json:"outcome"`
	Hashes  []string `json:"hashes"`
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
	t, ok := ParseMailDate(raw, time.UTC)
	if !ok {
		return strings.TrimSpace(raw)
	}
	return t.Format("2 Jan 2006")
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

// GroupActioned rolls actioned hashes into sender rows. It is not a kind tree.
func GroupActioned(actioned map[string]maildelete.ActionedInfo) []ActionedRow {
	type agg struct {
		sender  string
		outcome string
		hashes  []string
		dates   []string
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
		key := sender + "\x00" + info.Outcome
		g, ok := groups[key]
		if !ok {
			g = &agg{sender: sender, outcome: info.Outcome}
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
			Sender:  g.sender,
			Count:   len(g.hashes),
			Outcome: g.outcome,
			Hashes:  g.hashes,
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
		return out[i].Outcome < out[j].Outcome
	})
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
