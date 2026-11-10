package svg

import (
	"strings"
	"testing"
)

func TestBarChart_titleAxis(t *testing.T) {
	s := BarChart(200, 120, "T", "count", "cap", []string{"a"}, []int{3}, "currentColor")
	if !strings.Contains(s, "<title>T</title>") || !strings.Contains(s, "count") {
		t.Fatalf("missing title/axis: %s", s)
	}
}

func TestHistogramBars_title(t *testing.T) {
	b := []float64{0, 0.5, 1}
	c := []int{1, 2}
	s := HistogramBars(300, 140, "H", "x", "c", b, c, -1, "", "currentColor")
	if !strings.Contains(s, "<title>H</title>") || !strings.Contains(s, ">x<") {
		t.Fatal(s)
	}
}

func TestHeatmapGrid_hashAttrs(t *testing.T) {
	s := HeatmapGrid(400, 120, "H", "cap", []string{"w"}, []string{"Other"}, [][]int{{1}}, [][]string{{"hcell"}}, []string{"htheme"})
	if !strings.Contains(s, `data-hashes="hcell"`) || !strings.Contains(s, `data-hashes="htheme"`) {
		t.Fatal(s)
	}
}

func TestSparkline_hasPolylineAndThreePoints(t *testing.T) {
	got := Sparkline(100, 60, "S", []int{1, 2, 3}, []string{"a", "b", "c"})
	if !strings.Contains(got, "<title>S</title>") || !strings.Contains(got, `<polyline`) {
		t.Fatal(got)
	}
	if strings.Count(got, "points=") != 1 {
		t.Fatal("expected single polyline points attribute")
	}
}
