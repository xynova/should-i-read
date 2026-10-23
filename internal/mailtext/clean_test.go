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

func TestStripHTML_dropsStyleAsMetadata(t *testing.T) {
	raw := `<style>
@font-face {
font-family: 'Glassdoor Sans';
src: url(https://www.glassdoor.com/x.woff2) format('woff2');
}
</style>
<p>Apparently, I gave the wrong answer to a question.</p>`
	got := StripHTML(raw)
	if strings.Contains(strings.ToLower(got), "font-family") || strings.Contains(got, "@font-face") {
		t.Fatalf("style leaked: %q", got)
	}
	if !strings.Contains(got, "wrong answer") {
		t.Fatalf("want visible text: %q", got)
	}
}

func TestPlainFromRFC822_prefersPlainOverHTMLStyle(t *testing.T) {
	raw := []byte("From: a@example.com\r\n" +
		"To: b@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/alternative; boundary=abc\r\n" +
		"\r\n" +
		"--abc\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Community thread text\r\n" +
		"--abc\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<style>@font-face { font-family: X; }</style><p>HTML copy</p>\r\n" +
		"--abc--\r\n")
	got, err := PlainFromRFC822(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Community thread text") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "font-family") {
		t.Fatalf("html style leaked: %q", got)
	}
}
