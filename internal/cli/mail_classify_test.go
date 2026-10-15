package cli

import (
	"testing"

	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestIsClassifyUnavailable(t *testing.T) {
	t.Parallel()
	if isClassifyUnavailable(sirerr.New(sirerr.CodeUnavailable, "op", "polypus down")) {
		return
	}
	t.Fatal("expected unavailable")
}

func TestIsClassifyUnavailableFalse(t *testing.T) {
	t.Parallel()
	if isClassifyUnavailable(sirerr.New(sirerr.CodeFailed, "op", "other")) {
		t.Fatal("failed should not skip")
	}
	if isClassifyUnavailable(nil) {
		t.Fatal("nil should not skip")
	}
}

func TestMailReportWindow(t *testing.T) {
	t.Parallel()
	w, err := validateMailReportWindow("", "2026-09-01", "2026-09-08")
	if err != nil || w.Zero() {
		t.Fatalf("valid window: %v %v", w, err)
	}
	_, err = validateMailReportWindow("", "2026-09-01", "")
	if err == nil {
		t.Fatal("expected missing --to")
	}
	_, err = validateMailReportWindow("", "", "2026-09-08")
	if err == nil {
		t.Fatal("expected missing --from")
	}
	_, err = validateMailReportWindow("export.json", "2026-09-01", "2026-09-08")
	if err == nil {
		t.Fatal("expected --in conflict")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func TestReportListLimit(t *testing.T) {
	t.Parallel()
	window, err := pimdir.CreateDateWindow("2026-09-01", "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if reportListLimit(25, false, window) != 0 {
		t.Fatal("window without changed limit should be uncapped")
	}
	if reportListLimit(3, true, window) != 3 {
		t.Fatal("window with changed limit")
	}
	if reportListLimit(25, false, pimdir.DateWindow{}) != 25 {
		t.Fatal("no window uses limit")
	}
}
