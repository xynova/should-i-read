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

// alignAssignmentPathWithKind rewrites attach PATH to the three Essence KIND segments when the
// catalog path uses a hyphenated class id (notification-job-alert) or a semantic prefix.
func alignAssignmentPathWithKind(row *MessageRow) {
	if row == nil {
		return
	}
	kindSegs := catalog.ParseKindSegments(row.Kind)
	if len(kindSegs) != essenceKindLevels || strings.TrimSpace(row.TermID) == "" {
		return
	}
	if len(row.Path) == 0 {
		return
	}
	if pathEqualSegments(row.Path, kindSegs) {
		return
	}
	if !kindClassSlotMatches(row.Path, kindSegs) {
		return
	}
	row.Path = append([]string{}, kindSegs...)
}

func pathEqualSegments(path, kind []string) bool {
	if len(path) != len(kind) {
		return false
	}
	for i := range path {
		if strings.TrimSpace(path[i]) != strings.TrimSpace(kind[i]) {
			return false
		}
	}
	return true
}

// kindClassSlotMatches reports whether path is a KIND prefix, including family-class class ids.
func kindClassSlotMatches(path, kind []string) bool {
	if len(kind) != essenceKindLevels || len(path) < 1 || len(path) > essenceKindLevels {
		return false
	}
	family := strings.TrimSpace(kind[0])
	class := strings.TrimSpace(kind[1])
	leaf := strings.TrimSpace(kind[2])
	if family == "" || class == "" || leaf == "" {
		return false
	}
	if strings.TrimSpace(path[0]) != family {
		return false
	}
	if len(path) == 1 {
		return true
	}
	p1 := strings.TrimSpace(path[1])
	if p1 != class && p1 != family+"-"+class {
		return false
	}
	if len(path) == 2 {
		return true
	}
	return strings.TrimSpace(path[2]) == leaf
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
	if len(kindSegs) < essenceKindLevels {
		return sirerr.New(sirerr.CodeFailed, op, EssenceErrKindShort)
	}
	if len(kindSegs) > essenceKindLevels {
		return sirerr.New(sirerr.CodeFailed, op, EssenceErrKindLong)
	}
	if kindOrPathRepeatsParent(kindSegs) {
		return sirerr.New(sirerr.CodeFailed, op, EssenceErrKindAncestor)
	}
	if len(row.Path) < essenceKindLevels {
		return sirerr.New(sirerr.CodeFailed, op, AssignmentErrPathShort)
	}
	if len(row.Path) > essenceKindLevels {
		return sirerr.New(sirerr.CodeFailed, op, AssignmentErrPathDeep)
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
