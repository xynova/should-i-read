package mailtext

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
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
	const op = "mailtext.ExcerptFromRFC822"
	if opt.MaxRunes <= 0 {
		opt = DefaultOptions()
	}
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
	text, err := extractBody(mediaType, params, body)
	if err != nil {
		return "", err
	}
	return truncateRunes(strings.TrimSpace(text), opt.MaxRunes), nil
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
		if plain != "" {
			return plain, nil
		}
		return stripHTML(html), nil
	}
	if strings.HasPrefix(mediaType, "text/html") {
		return stripHTML(string(body)), nil
	}
	return string(body), nil
}

func walkMultipart(boundary string, body []byte) (plain, html string, err error) {
	const op = "mailtext.walkMultipart"
	r := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		p, err := r.NextPart()
		if err == io.EOF {
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

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
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
