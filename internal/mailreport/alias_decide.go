package mailreport

import (
	"context"
	"math"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

const aliasOwnerWorld = "Report-only: choose which catalog term owns this KIND path. Do not use a message body."

func decideAliasOwner(
	ctx context.Context,
	judge harness.Judge,
	cat *catalog.Catalog,
	alias, challenger, owner string,
	minScore float64,
) (winnerID string, ok bool) {
	if judge == nil || cat == nil || ctx == nil || ctx.Err() != nil {
		return "", false
	}
	alias = strings.TrimSpace(alias)
	challenger = strings.TrimSpace(challenger)
	owner = strings.TrimSpace(owner)
	if alias == "" || challenger == "" || owner == "" || challenger == owner {
		return "", false
	}
	options := []harness.PackedOption{
		termAliasOption(cat, challenger),
		termAliasOption(cat, owner),
		{Choice: harness.ChoiceSkip, Description: "skip: do not transfer alias ownership"},
	}
	in := harness.DecideIn{
		Text:         "KIND: " + alias,
		WorldContext: aliasOwnerWorld,
		Options:      options,
	}
	out, err := judge.Decide(ctx, in)
	if err != nil {
		return "", false
	}
	if out.Choice == harness.ChoiceSkip {
		return "", false
	}
	if math.IsNaN(out.Score) || math.IsInf(out.Score, 0) || out.Score < minScore {
		return "", false
	}
	if !strings.HasPrefix(out.Choice, harness.ChoicePrefixUse) {
		return "", false
	}
	id := strings.TrimPrefix(out.Choice, harness.ChoicePrefixUse)
	id = strings.TrimSpace(id)
	if id != challenger && id != owner {
		return "", false
	}
	return id, true
}

func termAliasOption(cat *catalog.Catalog, id string) harness.PackedOption {
	desc := catalog.Breadcrumb(cat, id)
	if rt, ok := cat.Lookup(id); ok && rt != nil {
		label := strings.TrimSpace(rt.Label)
		about := strings.TrimSpace(rt.Description)
		if label != "" {
			if desc != "" {
				desc += "; "
			}
			desc += "label: " + label
		}
		if about != "" {
			if desc != "" {
				desc += "; "
			}
			desc += "description: " + about
		}
	}
	return harness.PackedOption{
		Choice:      harness.ChoicePrefixUse + id,
		Description: desc,
	}
}
