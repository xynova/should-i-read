package mailtext

import (
	"strings"
	"testing"
)

func TestExcerptPrefersPlain(t *testing.T) {
	raw := []byte(`From: a@example.com
To: b@example.com
Subject: Test
Content-Type: multipart/alternative; boundary=abc

--abc
Content-Type: text/plain

Hello plain
--abc
Content-Type: text/html

<p>Hello <b>html</b></p>
--abc--
`)
	got, err := ExcerptFromRFC822(raw, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Hello plain") {
		t.Fatalf("got %q", got)
	}
}

func TestExcerptStripsHTML(t *testing.T) {
	raw := []byte(`From: a@example.com
To: b@example.com
Subject: Test
Content-Type: text/html

<p>Line <em>one</em></p>
`)
	got, err := ExcerptFromRFC822(raw, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if got != "Line one" {
		t.Fatalf("got %q", got)
	}
}
