package mailoverview

import (
	"strings"
	"time"
)

// ParseMailDate parses pimdir mail_summary.date (often RFC3339).
func ParseMailDate(s string, loc *time.Location) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.Local
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			if t.Location() == time.UTC {
				return t.In(loc), true
			}
			return t.In(loc), true
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", s[:10], loc); err == nil && len(s) >= 10 {
		return t, true
	}
	return time.Time{}, false
}

// MondayWeekStart returns the Monday of the week for t in loc.
func MondayWeekStart(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	tt := t.In(loc)
	wd := int(tt.Weekday())
	if wd == 0 {
		wd = 7
	}
	d := tt.AddDate(0, 0, 1-wd)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
}
