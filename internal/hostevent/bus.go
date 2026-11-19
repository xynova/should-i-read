package hostevent

import (
	"context"
	"sync"
	"time"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	SourceSync    = "sync"
	SourceEssence = "essence"
	subBuffer     = 32
)

// Event is one progress line from a host job.
type Event struct {
	Source string    `json:"source"`
	Kind   string    `json:"kind"`
	Line   string    `json:"line"`
	Done   int       `json:"done,omitempty"`
	Total  int       `json:"total,omitempty"`
	At     time.Time `json:"at"`
}

// Bus is an in-process fan-out of progress events. Nil receivers are no-ops.
type Bus struct {
	mu      sync.Mutex
	subs    map[int]chan Event
	next    int
	last    Event
	hasLast bool
	now     func() time.Time
}

// BusConfig wires a Bus.
type BusConfig struct {
	Now func() time.Time
}

// CreateBus builds an empty bus. Now is required.
func CreateBus(cfg BusConfig) (*Bus, error) {
	const op = "hostevent.CreateBus"
	if cfg.Now == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil clock")
	}
	return &Bus{subs: make(map[int]chan Event), now: cfg.Now}, nil
}

// Publish fans out ev. Slow subscribers are skipped. A nil bus is ignored.
func (b *Bus) Publish(ctx context.Context, ev Event) {
	if b == nil {
		return
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return
		}
	}
	if ev.At.IsZero() {
		ev.At = b.now().UTC()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last = ev
	b.hasLast = true
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Subscribe returns a buffered stream. The last event is replayed when present.
// The channel closes when ctx is done. A nil bus returns a closed channel.
func (b *Bus) Subscribe(ctx context.Context) <-chan Event {
	if b == nil || ctx == nil {
		ch := make(chan Event)
		close(ch)
		return ch
	}
	ch := make(chan Event, subBuffer)
	b.mu.Lock()
	id := b.next
	b.next++
	b.subs[id] = ch
	if b.hasLast {
		select {
		case ch <- b.last:
		default:
		}
	}
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		if _, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(ch)
		}
		b.mu.Unlock()
	}()
	return ch
}

// Last returns the most recent event when the bus has published at least once.
func (b *Bus) Last() (Event, bool) {
	if b == nil {
		return Event{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.hasLast {
		return Event{}, false
	}
	return b.last, true
}
