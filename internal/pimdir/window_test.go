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

func TestSplitDateWindow_monthStride7(t *testing.T) {
	t.Parallel()
	w, err := CreateDateWindow("2026-09-01", "2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	slices, err := SplitDateWindow(w, 7)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ from, to string }{
		{"2026-09-01", "2026-09-08"},
		{"2026-09-08", "2026-09-15"},
		{"2026-09-15", "2026-09-22"},
		{"2026-09-22", "2026-09-29"},
		{"2026-09-29", "2026-10-01"},
	}
	if len(slices) != len(want) {
		t.Fatalf("slices=%d want %d", len(slices), len(want))
	}
	for i, exp := range want {
		if slices[i].DateFrom() != exp.from || slices[i].DateTo() != exp.to {
			t.Fatalf("slice %d: %+v want %s %s", i, slices[i], exp.from, exp.to)
		}
	}
}

func TestSplitDateWindow_oneSlice(t *testing.T) {
	t.Parallel()
	w, err := CreateDateWindow("2026-09-01", "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	slices, err := SplitDateWindow(w, 7)
	if err != nil || len(slices) != 1 {
		t.Fatalf("slices=%v err=%v", slices, err)
	}
	if slices[0].DateFrom() != w.DateFrom() || slices[0].DateTo() != w.DateTo() {
		t.Fatal("expected single slice equal to parent")
	}
}

func TestSplitDateWindow_invalid(t *testing.T) {
	t.Parallel()
	w, _ := CreateDateWindow("2026-09-01", "2026-09-08")
	if _, err := SplitDateWindow(w, 0); err == nil {
		t.Fatal("expected error for days=0")
	}
	var z DateWindow
	if _, err := SplitDateWindow(z, 7); err == nil {
		t.Fatal("expected error for zero window")
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
