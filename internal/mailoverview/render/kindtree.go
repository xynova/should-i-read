package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xynova/should-i-read/internal/mailoverview"
)

// Kind tree folding: each branch shows at most kindTreeMaxShown children, and
// only those holding at least kindTreeMinShare of their parent and two or
// more messages. The rest fold into one "+N smaller" row.
const (
	kindTreeMaxShown = 8
	kindTreeMinShare = 0.02
)

// KindNode is one row of the kind tree: a KIND segment and its message count.
type KindNode struct {
	Label string
	Count int
	Depth int
	// ChildDepth indents this node's "+N smaller" row with its children.
	ChildDepth int
	// BarPct is the bar width against the largest top-level count, so every
	// level shares one scale.
	BarPct float64
	// Share is the node's part of its parent, or of the tab at the top level.
	Share string
	Open  bool
	// OneOff marks a top-level kind seen in only one message in this tab.
	OneOff bool
	Why    string

	Children []*KindNode
	// Small holds the folded tail of Children.
	Small         []*KindNode
	SmallMessages int
	// HashAttr is comma-joined object hashes for this node and its descendants.
	HashAttr string
	// SmallHashAttr is comma-joined hashes for the folded Small tail.
	SmallHashAttr string
}

type kindBranch struct {
	label  string
	count  int
	why    string
	hashes []string
	kids   map[string]*kindBranch
	order  []string
}

// buildKindTree turns full KIND paths into a folded tree, busiest first, and
// returns an unlabelled root whose children are the top-level kinds. The
// busiest top-level kind and its busiest child start open.
func buildKindTree(paths []mailoverview.KindPathRow) *KindNode {
	root := &kindBranch{kids: map[string]*kindBranch{}}
	total := 0
	for _, p := range paths {
		parts := mailoverview.KindPathParts(p.Path)
		if len(parts) == 0 || p.Count <= 0 {
			continue
		}
		total += p.Count
		cur := root
		for i, seg := range parts {
			next, ok := cur.kids[seg]
			if !ok {
				next = &kindBranch{label: seg, kids: map[string]*kindBranch{}}
				cur.kids[seg] = next
				cur.order = append(cur.order, seg)
			}
			next.count += p.Count
			if len(p.Hashes) > 0 {
				next.hashes = append(next.hashes, p.Hashes...)
			}
			if i == 0 && p.Why != "" {
				next.why = p.Why
			}
			cur = next
		}
	}
	if total == 0 {
		return nil
	}
	maxTop := 0
	for _, k := range root.kids {
		if k.count > maxTop {
			maxTop = k.count
		}
	}
	top := &KindNode{Count: total, Depth: -1, ChildDepth: 0}
	top.Children, top.Small, top.SmallMessages = foldKindBranch(root, total, maxTop, 0)
	top.SmallHashAttr = joinNodeHashes(top.Small)
	if len(top.Children) > 0 {
		first := top.Children[0]
		first.Open = true
		if len(first.Children) > 0 {
			first.Children[0].Open = true
		}
	}
	return top
}

func foldKindBranch(b *kindBranch, parentCount, maxTop, depth int) ([]*KindNode, []*KindNode, int) {
	kids := make([]*kindBranch, 0, len(b.order))
	for _, k := range b.order {
		kids = append(kids, b.kids[k])
	}
	sort.SliceStable(kids, func(i, j int) bool {
		if kids[i].count != kids[j].count {
			return kids[i].count > kids[j].count
		}
		return kids[i].label < kids[j].label
	})
	var shown, small []*KindNode
	smallMessages := 0
	for i, k := range kids {
		share := float64(k.count) / float64(parentCount)
		n := &KindNode{
			Label:      humanLabel(k.label),
			Count:      k.count,
			Depth:      depth,
			ChildDepth: depth + 1,
			BarPct:     float64(k.count) / float64(maxTop) * 100,
			Share:      shareLabel(share),
			OneOff:     depth == 0 && k.why != "",
			Why:        k.why,
			HashAttr:   mailoverview.JoinHashes(k.hashes),
		}
		n.Children, n.Small, n.SmallMessages = foldKindBranch(k, k.count, maxTop, depth+1)
		n.SmallHashAttr = joinNodeHashes(n.Small)
		if i == 0 || (i < kindTreeMaxShown && share >= kindTreeMinShare && k.count >= 2) {
			shown = append(shown, n)
			continue
		}
		small = append(small, n)
		smallMessages += k.count
	}
	// Folding a single row hides nothing worth hiding.
	if len(small) == 1 {
		shown = append(shown, small...)
		small, smallMessages = nil, 0
	}
	return shown, small, smallMessages
}

func joinNodeHashes(nodes []*KindNode) string {
	var hs []string
	for _, n := range nodes {
		if n == nil || n.HashAttr == "" {
			continue
		}
		hs = append(hs, strings.Split(n.HashAttr, ",")...)
	}
	return mailoverview.JoinHashes(hs)
}

func shareLabel(share float64) string {
	if share < 0.01 {
		return "<1%"
	}
	return fmt.Sprintf("%.0f%%", share*100)
}
