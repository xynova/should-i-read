package pimdir

import (
	"bytes"
	"net/mail"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// MailHeaders are RFC822 fields used for pre-AI classification.
type MailHeaders struct {
	From            string
	ReturnPath      string
	ListID          string
	ListUnsubscribe string
	AutoSubmitted   string
	Precedence      string
	Subject         string
}

// ParseMailHeaders reads selected headers from a raw message blob.
func ParseMailHeaders(raw []byte) (MailHeaders, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return MailHeaders{}, sirerr.Wrap(err, sirerr.CodeInvalid, "pimdir.ParseMailHeaders", "read message")
	}
	h := msg.Header
	return MailHeaders{
		From:            strings.TrimSpace(h.Get("From")),
		ReturnPath:      strings.TrimSpace(h.Get("Return-Path")),
		ListID:          strings.TrimSpace(h.Get("List-Id")),
		ListUnsubscribe: strings.TrimSpace(h.Get("List-Unsubscribe")),
		AutoSubmitted:   strings.TrimSpace(h.Get("Auto-Submitted")),
		Precedence:      strings.TrimSpace(h.Get("Precedence")),
		Subject:         strings.TrimSpace(h.Get("Subject")),
	}, nil
}

// FieldMap returns lowercase keys for taxonomy MatchFields on the host.
func (h MailHeaders) FieldMap(senderLine string) map[string]string {
	from := strings.TrimSpace(h.From)
	if from == "" {
		from = strings.TrimSpace(senderLine)
	}
	m := map[string]string{
		"from":             from,
		"return_path":      strings.TrimSpace(h.ReturnPath),
		"list_id":          strings.TrimSpace(h.ListID),
		"list_unsubscribe": strings.TrimSpace(h.ListUnsubscribe),
		"auto_submitted":   strings.TrimSpace(h.AutoSubmitted),
		"precedence":       strings.TrimSpace(h.Precedence),
		"subject":          strings.TrimSpace(h.Subject),
	}
	out := map[string]string{}
	for k, v := range m {
		if v != "" {
			out[k] = v
		}
	}
	return out
}
