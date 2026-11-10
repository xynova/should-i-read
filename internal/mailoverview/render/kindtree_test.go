package render

import (
	"testing"

	"github.com/xynova/should-i-read/internal/mailoverview"
)

func TestBuildKindTree_nestsSortsAndOpensBusiest(t *testing.T) {
	root := buildKindTree([]mailoverview.KindPathRow{
		{Path: "job > hiring > engineering", Count: 10, Hashes: []string{"j1"}},
		{Path: "notification > ci > workflow-failure", Count: 30, Hashes: []string{"w1", "w2"}},
		{Path: "notification > ci > pull-request", Count: 20, Hashes: []string{"p1"}},
		{Path: "notification > billing > payment-failure", Count: 5, Hashes: []string{"b1"}},
	})
	if root.Count != 65 || len(root.Children) != 2 {
		t.Fatalf("root %+v", root)
	}
	n := root.Children[0]
	if n.Label != "Notification" || n.Count != 55 || !n.Open || n.Share != "85%" || n.BarPct != 100 {
		t.Fatalf("top node %+v", n)
	}
	ci := n.Children[0]
	if ci.Label != "CI" || ci.Count != 50 || !ci.Open || ci.Depth != 1 {
		t.Fatalf("ci node %+v", ci)
	}
	if ci.Children[0].Label != "Workflow failure" || ci.Children[0].Share != "60%" || ci.Children[0].Depth != 2 {
		t.Fatalf("leaf %+v", ci.Children[0])
	}
	if ci.Children[0].HashAttr != "w1,w2" {
		t.Fatalf("leaf hashes %q", ci.Children[0].HashAttr)
	}
	if n.HashAttr != "w1,w2,p1,b1" {
		t.Fatalf("parent hashes %q", n.HashAttr)
	}
	if root.Children[1].Open {
		t.Fatal("only the busiest top-level kind starts open")
	}
}

func TestBuildKindTree_foldsTailAndMarksOneOff(t *testing.T) {
	paths := []mailoverview.KindPathRow{{Path: "notification > ci > workflow-failure", Count: 100, Hashes: []string{"n1"}}}
	for _, k := range []string{"a", "b", "c"} {
		paths = append(paths, mailoverview.KindPathRow{Path: k + " > x > y", Count: 1, Why: "only " + k, Hashes: []string{k}})
	}
	root := buildKindTree(paths)
	if len(root.Children) != 1 || len(root.Small) != 3 || root.SmallMessages != 3 {
		t.Fatalf("expected three one-offs folded: shown %d small %d", len(root.Children), len(root.Small))
	}
	if !root.Small[0].OneOff || root.Small[0].Why != "only a" || root.Small[0].Share != "<1%" {
		t.Fatalf("one-off %+v", root.Small[0])
	}
	if root.SmallHashAttr != "a,b,c" {
		t.Fatalf("folded tail hashes %q", root.SmallHashAttr)
	}
	if root.Small[0].Children[0].OneOff {
		t.Fatal("only the top-level kind carries the one-off badge")
	}
}

func TestBuildKindTree_singleTailRowStaysVisible(t *testing.T) {
	root := buildKindTree([]mailoverview.KindPathRow{
		{Path: "notification > ci > workflow-failure", Count: 100},
		{Path: "event > meetup > invite", Count: 1},
	})
	if len(root.Children) != 2 || len(root.Small) != 0 {
		t.Fatalf("one folded row should be shown instead: %d shown, %d small", len(root.Children), len(root.Small))
	}
}

func TestBuildKindTree_empty(t *testing.T) {
	if buildKindTree(nil) != nil {
		t.Fatal("no paths should give no tree")
	}
}
