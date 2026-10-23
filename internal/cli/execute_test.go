package cli

import (
	"context"
	"fmt"
	"testing"
)

func TestCommandExitCode_canceled(t *testing.T) {
	if got := commandExitCode(context.Canceled); got != 130 {
		t.Fatalf("bare cancel: %d", got)
	}
	if got := commandExitCode(fmt.Errorf("prefetch: %w", context.Canceled)); got != 130 {
		t.Fatalf("wrapped cancel: %d", got)
	}
}
