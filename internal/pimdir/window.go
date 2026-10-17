package pimdir

import (
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const dateLayout = "2006-01-02"

// DateWindow is a half-open UTC interval [From, To) on items.sort_key.
type DateWindow struct {
	From time.Time // UTC inclusive
	To   time.Time // UTC exclusive
}

// Zero reports whether no window was set.
func (w DateWindow) Zero() bool {
	return w.From.IsZero() && w.To.IsZero()
}

// FromRFC3339 returns the inclusive lower bound for SQL compare.
func (w DateWindow) FromRFC3339() string {
	return w.From.UTC().Format(time.RFC3339)
}

// ToRFC3339 returns the exclusive upper bound for SQL compare.
func (w DateWindow) ToRFC3339() string {
	return w.To.UTC().Format(time.RFC3339)
}

// DateFrom returns YYYY-MM-DD for the inclusive bound.
func (w DateWindow) DateFrom() string {
	return w.From.UTC().Format(dateLayout)
}

// DateTo returns YYYY-MM-DD for the exclusive bound.
func (w DateWindow) DateTo() string {
	return w.To.UTC().Format(dateLayout)
}

// CreateDateWindow parses YYYY-MM-DD UTC dates into [from, to).
func CreateDateWindow(from, to string) (DateWindow, error) {
	const op = "pimdir.CreateDateWindow"
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" {
		return DateWindow{}, sirerr.New(sirerr.CodeInvalid, op, "both from and to dates are required")
	}
	if strings.Contains(from, "T") || strings.Contains(from, "/") ||
		strings.Contains(to, "T") || strings.Contains(to, "/") {
		return DateWindow{}, sirerr.New(sirerr.CodeInvalid, op, "dates must be YYYY-MM-DD")
	}
	fromT, err := time.ParseInLocation(dateLayout, from, time.UTC)
	if err != nil {
		return DateWindow{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "parse from date")
	}
	toT, err := time.ParseInLocation(dateLayout, to, time.UTC)
	if err != nil {
		return DateWindow{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "parse to date")
	}
	if !fromT.Before(toT) {
		return DateWindow{}, sirerr.New(sirerr.CodeInvalid, op, "from date must be before to date")
	}
	return DateWindow{From: fromT, To: toT}, nil
}

// SplitDateWindow splits a half-open window into consecutive [from, to) slices of at most days length.
func SplitDateWindow(w DateWindow, days int) ([]DateWindow, error) {
	const op = "pimdir.SplitDateWindow"
	if w.Zero() {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty date window")
	}
	if days < 1 {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "days must be at least 1")
	}
	if !w.From.Before(w.To) {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "from must be before to")
	}
	var out []DateWindow
	cur := w.From.UTC()
	end := w.To.UTC()
	for cur.Before(end) {
		next := cur.AddDate(0, 0, days)
		if next.After(end) {
			next = end
		}
		out = append(out, DateWindow{From: cur, To: next})
		cur = next
	}
	return out, nil
}
