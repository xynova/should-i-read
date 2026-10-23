package mailtext

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// Options controls excerpt shaping for LLM input.
type Options struct {
	MaxRunes int
}

// DefaultOptions returns a sensible excerpt budget.
func DefaultOptions() Options {
	return Options{MaxRunes: 4000}
}

// ExcerptFromRFC822 extracts plain text (or stripped HTML) from a raw message.
func ExcerptFromRFC822(raw []byte, opt Options) (string, error) {
	if opt.MaxRunes <= 0 {
		opt = DefaultOptions()
	}
	text, err := PlainFromRFC822(raw)
	if err != nil {
		return "", err
	}
	return truncateRunes(strings.TrimSpace(text), opt.MaxRunes), nil
}

// PlainFromRFC822 returns decoded visible body text. Multipart prefers text/plain.
func PlainFromRFC822(raw []byte) (string, error) {
	const op = "mailtext.PlainFromRFC822"
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "parse message")
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		mediaType = "text/plain"
	}
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "read body")
	}
	body = decodeMIMEBody(msg.Header.Get("Content-Transfer-Encoding"), body)
	text, err := extractBody(mediaType, params, body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func extractBody(mediaType string, params map[string]string, body []byte) (string, error) {
	const op = "mailtext.extractBody"
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", sirerr.New(sirerr.CodeFailed, op, "multipart missing boundary")
		}
		plain, html, err := walkMultipart(boundary, body)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(plain) != "" {
			return strings.TrimSpace(plain), nil
		}
		return StripHTML(html), nil
	}
	if strings.HasPrefix(mediaType, "text/html") {
		return StripHTML(string(body)), nil
	}
	return string(body), nil
}

func walkMultipart(boundary string, body []byte) (plain, html string, err error) {
	const op = "mailtext.walkMultipart"
	r := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		p, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", "", sirerr.Wrap(err, sirerr.CodeFailed, op, "read part")
		}
		partType, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		partType = strings.ToLower(partType)
		partBody, err := io.ReadAll(p)
		if err != nil {
			return "", "", sirerr.Wrap(err, sirerr.CodeFailed, op, "read part body")
		}
		partBody = decodeMIMEBody(p.Header.Get("Content-Transfer-Encoding"), partBody)
		switch {
		case strings.HasPrefix(partType, "text/plain") && plain == "":
			plain = string(partBody)
		case strings.HasPrefix(partType, "text/html") && html == "":
			html = string(partBody)
		case strings.HasPrefix(partType, "multipart/"):
			_, subParams, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
			subPlain, subHTML, err := walkMultipart(subParams["boundary"], partBody)
			if err != nil {
				return "", "", err
			}
			if plain == "" && subPlain != "" {
				plain = subPlain
			}
			if html == "" && subHTML != "" {
				html = subHTML
			}
		}
	}
	return plain, html, nil
}

func decodeMIMEBody(enc string, body []byte) []byte {
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "quoted-printable":
		out, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(body)))
		if err != nil {
			return body
		}
		return out
	case "base64":
		out, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(bytes.TrimSpace(body))))
		if err != nil {
			return body
		}
		return out
	default:
		return body
	}
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	var n int
	for i := range s {
		if n == max {
			return s[:i] + "…"
		}
		n++
	}
	return s
}
