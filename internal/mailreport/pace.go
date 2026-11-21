package mailreport

import (
	"context"
	"sync"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

type paceCtxKey struct{}

const (
	paceMaxRPS   = 8.0
	paceMinRPS   = 0.5
	paceAddRPS   = 0.5
	paceSuccessN = 8
	paceCooldown = 2 * time.Second
	paceMultiply = 0.5
)

// PaceGate is one AIMD admit gate for a classify job's Chat fan-out.
type PaceGate struct {
	mu      sync.Mutex
	rps     float64
	min     float64
	max     float64
	next    time.Time
	success int
}

// CreatePaceGate builds a gate. A maxRPS of 0 or less disables pacing.
func CreatePaceGate(maxRPS float64) *PaceGate {
	if maxRPS <= 0 {
		return &PaceGate{rps: 0, min: 0, max: 0}
	}
	return &PaceGate{rps: maxRPS, min: paceMinRPS, max: maxRPS}
}

// WithPaceGate attaches gate to ctx. Existing gates are kept.
func WithPaceGate(ctx context.Context, gate *PaceGate) context.Context {
	if ctx == nil || gate == nil {
		return ctx
	}
	if PaceGateFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, paceCtxKey{}, gate)
}

// PaceGateFromContext returns the job gate, if any.
func PaceGateFromContext(ctx context.Context) *PaceGate {
	if ctx == nil {
		return nil
	}
	g, _ := ctx.Value(paceCtxKey{}).(*PaceGate)
	return g
}

// CurrentRPS is the live admit rate (0 means unlimited).
func (g *PaceGate) CurrentRPS() float64 {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rps
}

func (g *PaceGate) wait(ctx context.Context) error {
	const op = "mailreport.PaceGate.wait"
	if g == nil || g.max <= 0 {
		return nil
	}
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	for {
		g.mu.Lock()
		delay := time.Until(g.next)
		g.mu.Unlock()
		if delay <= 0 {
			g.mu.Lock()
			if time.Now().Before(g.next) {
				g.mu.Unlock()
				continue
			}
			if g.rps > 0 {
				g.next = time.Now().Add(time.Duration(float64(time.Second) / g.rps))
			}
			g.mu.Unlock()
			return nil
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (g *PaceGate) recordThrottle() {
	if g == nil || g.max <= 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rps *= paceMultiply
	if g.rps < g.min {
		g.rps = g.min
	}
	g.success = 0
	g.next = time.Now().Add(paceCooldown)
}

func (g *PaceGate) recordSuccess() {
	if g == nil || g.max <= 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.success++
	if g.success < paceSuccessN {
		return
	}
	g.success = 0
	g.rps += paceAddRPS
	if g.rps > g.max {
		g.rps = g.max
	}
}

func waitJobPace(ctx context.Context) error {
	g := PaceGateFromContext(ctx)
	if g == nil {
		return nil
	}
	return g.wait(ctx)
}

func noteChatPace(ctx context.Context, err error) {
	g := PaceGateFromContext(ctx)
	if g == nil {
		return
	}
	if err == nil {
		g.recordSuccess()
		return
	}
	if isUnavailable(err) {
		g.recordThrottle()
	}
}
