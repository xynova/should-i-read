package pimdir

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
)

// MessagePreview is decoded mail text for operator display.
type MessagePreview struct {
	Body      string
	Truncated bool
}

// DecodeMessagePreview extracts plain text from a raw RFC822 blob, capped at maxBytes.
func DecodeMessagePreview(raw []byte, maxBytes int) (MessagePreview, error) {
	if maxBytes <= 0 {
		maxBytes = 8192
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		body, truncated := truncateBytes(string(raw), maxBytes)
		return MessagePreview{Body: body, Truncated: truncated}, nil
	}
	mediaType, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if strings.HasPrefix(mediaType, "multipart/") {
		if body, truncated, ok := plainFromMultipart(msg.Body, params, maxBytes); ok {
			return MessagePreview{Body: body, Truncated: truncated}, nil
		}
		msg, err = mail.ReadMessage(bytes.NewReader(raw))
		if err != nil {
			body, truncated := truncateBytes(string(raw), maxBytes)
			return MessagePreview{Body: body, Truncated: truncated}, nil
		}
		mediaType, _, _ = mime.ParseMediaType(msg.Header.Get("Content-Type"))
	}
	payload, err := io.ReadAll(msg.Body)
	if err != nil {
		body, truncated := truncateBytes(string(raw), maxBytes)
		return MessagePreview{Body: body, Truncated: truncated}, nil
	}
	text := string(payload)
	if strings.HasPrefix(mediaType, "text/html") {
		text = stripSimpleHTML(text)
	}
	body, truncated := truncateBytes(text, maxBytes)
	return MessagePreview{Body: body, Truncated: truncated}, nil
}

func plainFromMultipart(body io.Reader, params map[string]string, maxBytes int) (string, bool, bool) {
	boundary := params["boundary"]
	if boundary == "" {
		return "", false, false
	}
	mr := multipart.NewReader(body, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false, false
		}
		pt, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			pt = "text/plain"
		}
		if pt != "text/plain" && pt != "text/html" {
			continue
		}
		payload, err := io.ReadAll(part)
		if err != nil {
			return "", false, false
		}
		text := string(payload)
		if pt == "text/html" {
			text = stripSimpleHTML(text)
		}
		body, truncated := truncateBytes(text, maxBytes)
		return body, truncated, true
	}
	return "", false, false
}

func truncateBytes(s string, maxBytes int) (string, bool) {
	if len(s) <= maxBytes {
		return s, false
	}
	return s[:maxBytes], true
}

func stripSimpleHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}
