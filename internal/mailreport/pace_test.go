package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestPaceGate_keepsOneGateOnContext(t *testing.T) {
	t.Parallel()
	first := CreatePaceGate(paceMaxRPS)
	ctx := WithPaceGate(context.Background(), first)
	ctx = WithPaceGate(ctx, CreatePaceGate(1))
	if PaceGateFromContext(ctx) != first {
		t.Fatal("expected the first gate to stay on context")
	}
}

func TestPaceGate_throttleSlowsRPS(t *testing.T) {
	t.Parallel()
	g := CreatePaceGate(paceMaxRPS)
	before := g.CurrentRPS()
	g.recordThrottle()
	after := g.CurrentRPS()
	if after >= before {
		t.Fatalf("rps did not drop: before=%v after=%v", before, after)
	}
}

func TestWaitJobPace_noGate(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := waitJobPace(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestNoteChatPace_unavailableIsThrottle(t *testing.T) {
	t.Parallel()
	g := CreatePaceGate(paceMaxRPS)
	ctx := WithPaceGate(context.Background(), g)
	before := g.CurrentRPS()
	noteChatPace(ctx, sirerr.New(sirerr.CodeUnavailable, "test", "429"))
	if g.CurrentRPS() >= before {
		t.Fatal("expected throttle to drop rps")
	}
}
