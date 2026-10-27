package mailreport

import (
	"context"
	"testing"

	"github.com/xynova/should-i-read/internal/config"
)

type vectorStubEmbedder struct {
	byText map[string][]float64
}

func (v *vectorStubEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, t := range texts {
		if v.byText != nil {
			if vec, ok := v.byText[t]; ok {
				out[i] = vec
				continue
			}
		}
		out[i] = []float64{1, 0, 0}
	}
	return out, nil
}

func TestMeasureDescriptionCosine_groundedHighRectangularLow(t *testing.T) {
	email := "GitHub Actions workflow failed on main"
	grounded := "CI failure notification for a GitHub repository workflow"
	rectangular := "Rectangular"
	emb := &vectorStubEmbedder{byText: map[string][]float64{
		email:       {1, 0, 0},
		grounded:    {0.95, 0.05, 0},
		rectangular: {0, 1, 0},
	}}
	ctx := context.Background()
	high, err := MeasureDescriptionCosine(ctx, emb, email, grounded)
	if err != nil {
		t.Fatal(err)
	}
	low, err := MeasureDescriptionCosine(ctx, emb, email, rectangular)
	if err != nil {
		t.Fatal(err)
	}
	min := 0.55
	if high < min {
		t.Fatalf("grounded cosine %v want >= %v", high, min)
	}
	if low >= min {
		t.Fatalf("rectangular cosine %v want < %v", low, min)
	}
}

func TestRunner_reportDescriptionCosine_setsRowFields(t *testing.T) {
	email := "body"
	about := "about"
	emb := &vectorStubEmbedder{byText: map[string][]float64{
		email: {1, 0, 0},
		about: {0, 1, 0},
	}}
	runner := &Runner{
		descriptionCosineMode: config.DescriptionCosineModeReport,
		descriptionCosineMin:  0.55,
		seats:                 Seats{Embedder: emb},
	}
	row := &MessageRow{About: about, Cosine: 0.99}
	runner.reportDescriptionCosine(context.Background(), row, Item{Body: email})
	if row.Cosine != 0.99 {
		t.Fatalf("attach cosine changed: %v", row.Cosine)
	}
	if row.DescriptionCosine != 0 {
		t.Fatalf("orthogonal cosine %v", row.DescriptionCosine)
	}
	if !row.DescriptionCosineBelowMin {
		t.Fatal("expected below min")
	}
}
