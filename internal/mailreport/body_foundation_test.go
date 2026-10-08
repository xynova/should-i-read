package mailreport

import (
	"strings"
	"testing"
)

func TestFoundationBody_stripsQuotedReply(t *testing.T) {
	body := "Please review the doc.\n\nOn Mon, Jan 1, 2024 at 9:00 AM Bob <bob@example.com> wrote:\n> old thread\n"
	got := FoundationBody(body)
	if !strings.Contains(got, "Please review the doc") {
		t.Fatalf("want latest line kept: %q", got)
	}
	if strings.Contains(got, "old thread") {
		t.Fatalf("want quoted history removed: %q", got)
	}
}

func TestAuthorMessageBody_keepsSubjectHeader(t *testing.T) {
	body := "Latest only.\n\nOn Mon, Jan 1, 2024 at 9:00 AM Bob <bob@example.com> wrote:\n> quoted line\n"
	in := "Subject: Hello\nFrom: a@b.com\n\n" + body
	got := AuthorMessageBody(in)
	if !strings.HasPrefix(got, "Subject: Hello") {
		t.Fatalf("header missing: %q", got)
	}
	if strings.Contains(got, "quoted line") {
		t.Fatalf("want quoted body removed: %q", got)
	}
	if !strings.Contains(got, "Latest only") {
		t.Fatalf("want latest body: %q", got)
	}
}
