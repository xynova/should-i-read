package setup

import (
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// ValidateGmailClientID checks a Google OAuth client id shape without logging it.
func ValidateGmailClientID(id string) error {
	const op = "setup.ValidateGmailClientID"
	id = strings.TrimSpace(id)
	if id == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "gmail client id is empty")
	}
	if len(id) < 12 {
		return sirerr.New(sirerr.CodeInvalid, op, "gmail client id looks too short")
	}
	if strings.ContainsAny(id, " \t\n\r") {
		return sirerr.New(sirerr.CodeInvalid, op, "gmail client id must not contain whitespace")
	}
	return nil
}

// GmailConsoleURL is the Google Cloud credentials console.
const GmailConsoleURL = "https://console.cloud.google.com/apis/credentials"

// GmailAPIURL is the Gmail API library page (enable API).
const GmailAPIURL = "https://console.cloud.google.com/apis/library/gmail.googleapis.com"
