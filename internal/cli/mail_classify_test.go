package cli

import (
	"errors"
	"testing"

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
	if isClassifyUnavailable(errors.New("plain")) {
		t.Fatal("plain should not skip")
	}
}
