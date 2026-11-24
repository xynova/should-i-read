package mailtext

import (
	"strings"
	"testing"
)

func TestHTMLFromRFC822_multipartReturnsHTMLPart(t *testing.T) {
	raw := []byte("From: a@example.com\r\n" +
		"To: b@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: multipart/alternative; boundary=abc\r\n" +
		"\r\n" +
		"--abc\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"<<https://c.gle/track>> plain only\r\n" +
		"--abc\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<p>HTML copy</p>\r\n" +
		"--abc--\r\n")
	got, err := HTMLFromRFC822(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<p>HTML copy</p>") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "c.gle") {
		t.Fatalf("should not prefer plain: %q", got)
	}
}

func TestHTMLFromRFC822_rootHTML(t *testing.T) {
	raw := []byte("From: a@example.com\r\n" +
		"Subject: T\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<h1>Only HTML</h1>\r\n")
	got, err := HTMLFromRFC822(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<h1>Only HTML</h1>") {
		t.Fatalf("got %q", got)
	}
}

func TestSafePreviewHTML_keepsContentImagesBlocksTracking(t *testing.T) {
	in := `<html><head></head><body>` +
		`<a href="https://c.gle/x">x</a>` +
		`<img src="https://lh3.googleusercontent.com/logo.png" alt="logo">` +
		`<img src="https://c.gle/pixel" width="1" height="1">` +
		`<form action="https://evil/submit"></form>` +
		`<style>body { background: url(https://lh3.googleusercontent.com/hero.png); }</style>` +
		`</body></html>`
	got := SafePreviewHTML(in)
	if !strings.Contains(got, "Content-Security-Policy") {
		t.Fatal("missing CSP meta")
	}
	if !strings.Contains(got, "img-src https:") {
		t.Fatal("CSP must allow content images")
	}
	if !strings.Contains(got, "padding:0.5rem 0 2rem !important") {
		t.Fatal("preview chrome must keep vertical padding the mail CSS cannot zero")
	}
	if strings.Contains(got, "https://c.gle") {
		t.Fatalf("tracking URL leaked: %q", got)
	}
	if !strings.Contains(got, "https://lh3.googleusercontent.com/logo.png") {
		t.Fatalf("content image stripped: %q", got)
	}
	if !strings.Contains(got, "https://lh3.googleusercontent.com/hero.png") {
		t.Fatalf("content css image stripped: %q", got)
	}
	if strings.Contains(got, "https://evil/submit") {
		t.Fatalf("form action leaked: %q", got)
	}
}

func TestSafePreviewHTML_keepsLayoutSpacers(t *testing.T) {
	in := `<img src="https://www.gstatic.com/gumdrop/files/spacer-11.gif" height="60" width="1">`
	got := SafePreviewHTML(in)
	if !strings.Contains(got, "spacer-11.gif") {
		t.Fatalf("layout spacer stripped: %q", got)
	}
	if !strings.Contains(got, `img:not([width="1"]):not([height="1"])`) {
		t.Fatal("chrome must not apply height:auto to HTML spacers")
	}
	if strings.Contains(got, `width:1px;height:1px`) {
		t.Fatal("chrome must keep HTML spacer height attributes")
	}
}

func TestSafePreviewHTML_rewritesProtocolRelative(t *testing.T) {
	in := `<img src="//lh3.googleusercontent.com/logo.png" alt="logo">`
	got := SafePreviewHTML(in)
	if !strings.Contains(got, `src="https://lh3.googleusercontent.com/logo.png"`) {
		t.Fatalf("protocol-relative image not rewritten: %q", got)
	}
	if strings.Contains(got, `src="//lh3`) {
		t.Fatalf("bare protocol-relative leaked: %q", got)
	}
}
