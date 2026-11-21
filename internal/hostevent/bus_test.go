package hostevent

import (
	"context"
	"testing"
	"time"
)

func TestPublishNilBus(t *testing.T) {
	t.Parallel()
	var b *Bus
	b.Publish(context.Background(), Event{Line: "x"})
	if _, ok := b.Last(); ok {
		t.Fatal("nil bus must not store last")
	}
}

func testBus(t *testing.T) *Bus {
	t.Helper()
	b, err := CreateBus(BusConfig{Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCreateBusRejectsNilClock(t *testing.T) {
	t.Parallel()
	if _, err := CreateBus(BusConfig{}); err == nil {
		t.Fatal("expected nil clock")
	}
}

func TestSubscribeReceivesPublish(t *testing.T) {
	t.Parallel()
	b := testBus(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := b.Subscribe(ctx)
	b.Publish(ctx, Event{Source: SourceSync, Line: "Scanning imap 1/9", Done: 1, Total: 9})
	select {
	case ev := <-ch:
		if ev.Line != "Scanning imap 1/9" || ev.Done != 1 || ev.Total != 9 {
			t.Fatalf("%+v", ev)
		}
		if ev.At.IsZero() {
			t.Fatal("missing timestamp")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestSubscribeReplaysLast(t *testing.T) {
	t.Parallel()
	b := testBus(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.Publish(ctx, Event{Line: "Essence 4", Source: SourceEssence})
	ch := b.Subscribe(ctx)
	select {
	case ev := <-ch:
		if ev.Line != "Essence 4" {
			t.Fatalf("%+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestPublishCanceledContextDropped(t *testing.T) {
	t.Parallel()
	b := testBus(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b.Publish(ctx, Event{Line: "nope"})
	if _, ok := b.Last(); ok {
		t.Fatal("canceled publish must not store last")
	}
}

func TestSubscribeNilContextClosed(t *testing.T) {
	t.Parallel()
	b := testBus(t)
	var ctx context.Context
	ch := b.Subscribe(ctx)
	if _, ok := <-ch; ok {
		t.Fatal("nil context must close the subscriber")
	}
}
