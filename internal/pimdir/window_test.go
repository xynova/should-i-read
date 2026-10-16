package pimdir

import (
	"strings"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestCreateDateWindow(t *testing.T) {
	t.Parallel()
	w, err := CreateDateWindow("2026-09-01", "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if w.FromRFC3339() != "2026-09-01T00:00:00Z" || w.ToRFC3339() != "2026-09-08T00:00:00Z" {
		t.Fatalf("bounds: from=%s to=%s", w.FromRFC3339(), w.ToRFC3339())
	}
	if w.DateFrom() != "2026-09-01" || w.DateTo() != "2026-09-08" {
		t.Fatalf("dates: %s %s", w.DateFrom(), w.DateTo())
	}
	if w.Zero() {
		t.Fatal("expected non-zero window")
	}
}

func TestCreateDateWindowLeapAndYearBoundary(t *testing.T) {
	t.Parallel()
	w, err := CreateDateWindow("2024-02-28", "2024-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if w.DateFrom() != "2024-02-28" || w.DateTo() != "2024-03-01" {
		t.Fatalf("leap: %+v", w)
	}
	w2, err := CreateDateWindow("2026-12-28", "2027-01-04")
	if err != nil {
		t.Fatal(err)
	}
	if w2.DateTo() != "2027-01-04" {
		t.Fatalf("year boundary: %s", w2.DateTo())
	}
}

func TestCreateDateWindowInvalid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to string
	}{
		{"2026-09-01", ""},
		{"", "2026-09-08"},
		{"2026-09-08", "2026-09-08"},
		{"2026-09-09", "2026-09-08"},
		{"2026-09-01T00:00:00Z", "2026-09-08"},
		{"09/01/2026", "2026-09-08"},
		{"2026-09-01", "09/08/2026"},
	}
	for _, c := range cases {
		_, err := CreateDateWindow(c.from, c.to)
		if err == nil {
			t.Fatalf("expected error for %q %q", c.from, c.to)
		}
		code, ok := sirerr.AsCode(err)
		if !ok || code != sirerr.CodeInvalid {
			t.Fatalf("code for %q %q: %v", c.from, c.to, err)
		}
		if strings.Contains(err.Error(), "--") {
			t.Fatalf("pimdir error must not name CLI flags: %v", err)
		}
	}
}

func TestDateWindowZero(t *testing.T) {
	t.Parallel()
	var w DateWindow
	if !w.Zero() {
		t.Fatal("zero value should be zero window")
	}
	w.From = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if w.Zero() {
		t.Fatal("partial window is not zero")
	}
}
