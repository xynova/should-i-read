package mailreport

import (
	"context"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hostevent"
)

func TestPolypusEssencerPublishEssence(t *testing.T) {
	t.Parallel()
	bus, err := hostevent.CreateBus(hostevent.BusConfig{Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := bus.Subscribe(ctx)
	e := &polypusEssencer{events: bus}
	e.publishEssence(ctx)
	e.publishEssence(ctx)
	want := []string{"Essence 1", "Essence 2"}
	for i, line := range want {
		select {
		case ev := <-ch:
			if ev.Source != hostevent.SourceEssence || ev.Line != line || ev.Done != i+1 {
				t.Fatalf("event %d %+v", i, ev)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for %s", line)
		}
	}
}

func TestPolypusEssencerPublishEssenceNilBus(t *testing.T) {
	t.Parallel()
	e := &polypusEssencer{}
	e.publishEssence(context.Background())
}
