package setup

import (
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// ValidateOutlookClientID checks an Entra application (client) id shape.
func ValidateOutlookClientID(id string) error {
	const op = "setup.ValidateOutlookClientID"
	id = strings.TrimSpace(id)
	if id == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "outlook client id is empty")
	}
	if len(id) < 12 {
		return sirerr.New(sirerr.CodeInvalid, op, "outlook client id looks too short")
	}
	if strings.ContainsAny(id, " \t\n\r") {
		return sirerr.New(sirerr.CodeInvalid, op, "outlook client id must not contain whitespace")
	}
	return nil
}

// OutlookPortalURL is the Entra app registrations blade.
const OutlookPortalURL = "https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade"
