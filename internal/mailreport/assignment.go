package mailreport

import (
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// syncRowPathFromCatalog fills row.Path from the resolved leaf when attach breadcrumb
// results only carry the consumed KIND prefix (taxonomy harness finishBreadcrumb).
func syncRowPathFromCatalog(cat *catalog.Catalog, row *MessageRow) {
	if cat == nil || row == nil {
		return
	}
	id := strings.TrimSpace(row.TermID)
	if id == "" {
		return
	}
	rt, ok := cat.PreferLeaf(id)
	if !ok || rt == nil {
		return
	}
	if len(rt.Segments) > 0 {
		row.Path = append([]string{}, rt.Segments...)
		return
	}
	if len(rt.Path) > 0 {
		row.Path = append([]string{}, rt.Path...)
	}
}

// alignAssignmentPathWithKind extends a prefix-only attach path to match Essence KIND segments.
func alignAssignmentPathWithKind(row *MessageRow) {
	if row == nil {
		return
	}
	kindSegs := catalog.ParseKindSegments(row.Kind)
	if len(kindSegs) < 3 || strings.TrimSpace(row.TermID) == "" {
		return
	}
	if len(row.Path) >= len(kindSegs) {
		return
	}
	if !pathIsPrefixOf(row.Path, kindSegs) {
		return
	}
	row.Path = append([]string{}, kindSegs...)
}

func pathIsPrefixOf(path, full []string) bool {
	if len(path) > len(full) {
		return false
	}
	for i := range path {
		if strings.TrimSpace(path[i]) != strings.TrimSpace(full[i]) {
			return false
		}
	}
	return true
}

func validateAssignmentPath(row MessageRow) error {
	const op = "mailreport.validateAssignmentPath"
	kindSegs := catalog.ParseKindSegments(row.Kind)
	if len(kindSegs) == 0 {
		return nil
	}
	if len(kindSegs) < 3 {
		return sirerr.New(sirerr.CodeFailed, op, EssenceErrKindShort)
	}
	if kindOrPathRepeatsParent(kindSegs) {
		return sirerr.New(sirerr.CodeFailed, op, EssenceErrKindAncestor)
	}
	if len(row.Path) < 3 {
		return sirerr.New(sirerr.CodeFailed, op, AssignmentErrPathShort)
	}
	if kindOrPathRepeatsParent(row.Path) {
		return sirerr.New(sirerr.CodeFailed, op, AssignmentErrPathAncestor)
	}
	return nil
}

func kindOrPathRepeatsParent(segs []string) bool {
	for i := 1; i < len(segs); i++ {
		parent := strings.TrimSpace(segs[i-1])
		child := strings.TrimSpace(segs[i])
		if parent == "" || child == "" {
			continue
		}
		if child == parent {
			return true
		}
		if strings.HasPrefix(child, parent+"-") {
			return true
		}
	}
	return false
}

func essenceKindAncestorFromErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), EssenceErrKindAncestor)
}
