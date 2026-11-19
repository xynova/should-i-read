package mailjob

import (
	"strconv"
	"strings"
)

// Mailbox returns the Himalaya -m folder for an item.
func Mailbox(it Item) string {
	if folder := strings.TrimSpace(it.Collection); folder != "" {
		return folder
	}
	return strings.TrimSpace(it.Folder)
}

// EnvelopeID prefers pimdir seq. Himalaya message ids are decimal numbers;
// RFC Message-ID strings in link_id are rejected.
func EnvelopeID(it Item) string {
	if it.Seq > 0 {
		return strconv.FormatInt(it.Seq, 10)
	}
	if id := numericMessageID(it.EnvelopeID); id != "" {
		return id
	}
	return numericMessageID(it.LinkID)
}

func numericMessageID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return id
}
