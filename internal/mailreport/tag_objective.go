package mailreport

import (
	"github.com/behaviorengineering/taxonomy/pkg/tags"
)

var mailTagDenyIDs = []string{
	"rectangular", "square", "circle", "triangle",
	"subject", "email", "body", "line", "message", "sender", "content", "text",
}

// MailTagCatalogConfig builds tag catalog config with host deny list and XDG paths.
func MailTagCatalogConfig(registerPath, deprecationsPath, seedPath string) tags.Config {
	return tags.Config{
		RegisterPath:     registerPath,
		DeprecationsPath: deprecationsPath,
		SeedPath:         seedPath,
		Deny:             mailTagDenyIDs,
	}
}

func mailTagObjective() tags.Objective {
	return tags.Objective{
		WorldContext: tagCurateWorld,
		Purpose:      tagKeepDescription,
		NotPurpose:   tagDropDescription,
		TokenHint:    tagTokenUseLine,
		KeepOwn:      tagKeepOwnDescription,
	}
}

func mailTagDenySet() map[string]struct{} {
	out := make(map[string]struct{}, len(mailTagDenyIDs))
	for _, id := range mailTagDenyIDs {
		out[id] = struct{}{}
	}
	return out
}
