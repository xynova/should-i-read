package pimdir

import (
	"github.com/xynova/should-i-read/internal/mailtext"
)

const defaultPreviewMaxBytes = 8192

// MessagePreview is decoded mail text for operator display.
type MessagePreview struct {
	Body      string
	Truncated bool
}

// DecodeMessagePreview extracts visible plain text from a raw RFC822 blob, capped at maxBytes.
func DecodeMessagePreview(raw []byte, maxBytes int) (MessagePreview, error) {
	if maxBytes <= 0 {
		maxBytes = defaultPreviewMaxBytes
	}
	text, err := mailtext.PlainFromRFC822(raw)
	if err != nil {
		text = mailtext.StripHTML(string(raw))
	}
	body, truncated := truncateBytes(text, maxBytes)
	return MessagePreview{Body: body, Truncated: truncated}, nil
}

func truncateBytes(s string, maxBytes int) (string, bool) {
	if len(s) <= maxBytes {
		return s, false
	}
	return s[:maxBytes], true
}
