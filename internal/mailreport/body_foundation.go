package mailreport

import (
	"strings"

	emailreplyparser "github.com/web-ridge/email-reply-parser"
)

// FoundationBody keeps the latest visible reply from a plain-text mail body.
func FoundationBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	return strings.TrimSpace(emailreplyparser.Parse(body))
}

// splitFormatItemText separates Subject/From header lines from the body in formatItemText output.
func splitFormatItemText(formatted string) (header, body string) {
	formatted = strings.TrimSpace(formatted)
	if formatted == "" {
		return "", ""
	}
	idx := strings.Index(formatted, "\n\n")
	if idx < 0 {
		return "", formatted
	}
	return strings.TrimSpace(formatted[:idx]), strings.TrimSpace(formatted[idx+2:])
}

// joinFormatItemText recombines header and body for Author prompts.
func joinFormatItemText(header, body string) string {
	header = strings.TrimSpace(header)
	body = strings.TrimSpace(body)
	if header == "" {
		return body
	}
	if body == "" {
		return header
	}
	return header + "\n\n" + body
}
