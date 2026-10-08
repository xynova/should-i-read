package mailreport

import (
	"strings"
	"testing"
)

func TestDenoiseBody_dropsUnsubscribeLine(t *testing.T) {
	in := "Thanks for subscribing.\n\nUnsubscribe here\n\nReal content."
	got := DenoiseBody(in)
	if strings.Contains(strings.ToLower(got), "unsubscribe") {
		t.Fatalf("want unsubscribe line removed: %q", got)
	}
	if !strings.Contains(got, "Real content") {
		t.Fatalf("want content kept: %q", got)
	}
}

func TestDenoiseBody_decodesEntities(t *testing.T) {
	got := DenoiseBody("Price&nbsp;drop")
	if !strings.Contains(got, "Price drop") {
		t.Fatalf("want decoded entity: %q", got)
	}
}
