package mailreport

import (
	"fmt"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const authorSystemPrompt = `Reply with JSON only. No markdown fences or prose outside JSON.

Choose exactly one draft:
- kind "alias": map this message to an existing leaf id. Include leafId (must be one of the listed leaf ids) and alias (short synonym phrase).
- kind "new_leaf": only when no listed leaf fits. Include id (kebab-case), parent (one of the listed branch ids), label, description.

Prefer alias over new_leaf. Do not invent leaf ids outside the list.`

func buildAuthorPrompt(in harness.DraftIn, leafLines string, messageText string) string {
	var b strings.Builder
	b.WriteString("World context:\n")
	b.WriteString(in.WorldContext)
	b.WriteString("\n\nMessage:\n")
	b.WriteString(messageText)
	b.WriteString("\n\nReason for author:\n")
	b.WriteString(in.Reason)
	b.WriteString("\n\nAllowed leaf ids (choose alias leafId from this list only):\n")
	b.WriteString(leafLines)
	b.WriteString("\n\nBranches (new_leaf parent must be one of these):\n")
	for _, p := range in.Parents {
		fmt.Fprintf(&b, "- parent=%s label=%s\n", p.ParentID, p.Label)
	}
	return b.String()
}

func validateAuthorDraft(cat *catalog.Catalog, parents []harness.PackedOption, parsed authorJSON) error {
	const op = "mailreport.validateAuthorDraft"
	kind := strings.TrimSpace(parsed.Kind)
	switch kind {
	case harness.DraftKindAlias:
		leafID := strings.TrimSpace(parsed.LeafID)
		if leafID == "" {
			return sirerr.New(sirerr.CodeFailed, op, "alias draft missing leafId")
		}
		if cat == nil || !cat.IsLeaf(leafID) {
			return sirerr.New(sirerr.CodeFailed, op, "leafId not in catalog").With("leafId", leafID)
		}
		if !leafInAuthorScope(cat, parents, leafID) {
			return sirerr.New(sirerr.CodeFailed, op, "leafId not in author scope").With("leafId", leafID)
		}
		if strings.TrimSpace(parsed.Alias) == "" {
			return sirerr.New(sirerr.CodeFailed, op, "alias draft missing alias")
		}
		return nil
	case harness.DraftKindNewLeaf:
		id := strings.TrimSpace(parsed.ID)
		parent := strings.TrimSpace(parsed.Parent)
		if id == "" || parent == "" {
			return sirerr.New(sirerr.CodeFailed, op, "new_leaf draft missing id or parent")
		}
		if !isKebabID(id) {
			return sirerr.New(sirerr.CodeFailed, op, "new_leaf id must be kebab-case").With("id", id)
		}
		if !parentInAuthorScope(cat, parents, parent) {
			return sirerr.New(sirerr.CodeFailed, op, "parent not in author scope").With("parent", parent)
		}
		if strings.TrimSpace(parsed.Label) == "" {
			return sirerr.New(sirerr.CodeFailed, op, "new_leaf draft missing label")
		}
		return nil
	default:
		return sirerr.New(sirerr.CodeFailed, op, "unknown draft kind").With("kind", kind)
	}
}

func leafInAuthorScope(cat *catalog.Catalog, parents []harness.PackedOption, leafID string) bool {
	if cat == nil {
		return false
	}
	if len(parents) == 0 {
		_, ok := cat.Lookup(leafID)
		return ok && cat.IsLeaf(leafID)
	}
	for _, p := range parents {
		root := strings.TrimSpace(p.ParentID)
		if root == "" {
			continue
		}
		for _, id := range cat.DescendantLeaves(root) {
			if id == leafID {
				return true
			}
		}
	}
	return false
}

func parentInAuthorScope(cat *catalog.Catalog, parents []harness.PackedOption, parentID string) bool {
	if len(parents) > 0 {
		for _, p := range parents {
			if strings.TrimSpace(p.ParentID) == parentID {
				return true
			}
		}
		return false
	}
	if cat == nil {
		return false
	}
	rt, ok := cat.Lookup(parentID)
	if !ok {
		return false
	}
	return !cat.IsLeaf(rt.ID)
}

func isKebabID(id string) bool {
	if id == "" || strings.Contains(id, " ") {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '-' {
			continue
		}
		return false
	}
	return !strings.HasPrefix(id, "-") && !strings.HasSuffix(id, "-")
}
