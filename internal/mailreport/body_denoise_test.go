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

func TestDenoiseBody_dropsCSSMetadataLines(t *testing.T) {
	in := "@font-face {\nfont-family: 'Glassdoor Sans';\nsrc: url(https://example.com/x.woff2) format('woff2');\n}\nApparently I gave the wrong answer."
	got := DenoiseBody(in)
	if strings.Contains(strings.ToLower(got), "font-family") || strings.Contains(got, "@font-face") {
		t.Fatalf("css leaked: %q", got)
	}
	if !strings.Contains(got, "wrong answer") {
		t.Fatalf("want content kept: %q", got)
	}
}
