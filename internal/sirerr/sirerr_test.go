package sirerr_test

import (
	"errors"
	"testing"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestExitCodeMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code sirerr.Code
		want int
	}{
		{sirerr.CodeInvalid, 2},
		{sirerr.CodeNotFound, 3},
		{sirerr.CodeAuth, 4},
		{sirerr.CodeNetwork, 5},
		{sirerr.CodeAI, 6},
		{sirerr.CodeFailed, 1},
		{sirerr.CodeUnavailable, 5},
		{sirerr.CodeNeedsReview, 7},
	}
	for _, tc := range cases {
		got := sirerr.ExitCode(sirerr.New(tc.code, "op", "msg"))
		if got != tc.want {
			t.Fatalf("%s => %d want %d", tc.code, got, tc.want)
		}
	}
	if sirerr.ExitCode(nil) != 0 {
		t.Fatal("nil should be 0")
	}
	if sirerr.ExitCode(errors.New("plain")) != 1 {
		t.Fatal("plain error should be 1")
	}
}
