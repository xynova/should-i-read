package mailreport

import (
	"context"
	"math"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// MeasureDescriptionCosine embeds email body text and Essence ABOUT, then returns cosine similarity.
func MeasureDescriptionCosine(ctx context.Context, embedder harness.Embedder, emailBody, about string) (float64, error) {
	const op = "mailreport.MeasureDescriptionCosine"
	about = strings.TrimSpace(about)
	body := strings.TrimSpace(emailBody)
	if about == "" || body == "" {
		return 0, nil
	}
	if embedder == nil {
		return 0, sirerr.New(sirerr.CodeInvalid, op, "nil embedder")
	}
	vecs, err := embedder.Embed(ctx, []string{body, about})
	if err != nil {
		return 0, sirerr.Wrap(err, sirerr.CodeFailed, op, "embed")
	}
	if len(vecs) < 2 {
		return 0, sirerr.New(sirerr.CodeFailed, op, "short embed response")
	}
	return cosineSimilarity(vecs[0], vecs[1]), nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func (r *Runner) reportDescriptionCosine(ctx context.Context, row *MessageRow, item Item) {
	if r == nil || row == nil || r.descriptionCosineMode == config.DescriptionCosineModeOff {
		return
	}
	if strings.TrimSpace(row.About) == "" || strings.TrimSpace(item.Body) == "" {
		return
	}
	if r.seats.Embedder == nil {
		return
	}
	score, err := MeasureDescriptionCosine(ctx, r.seats.Embedder, item.Body, row.About)
	if err != nil {
		return
	}
	row.DescriptionCosine = score
	min := r.descriptionCosineMin
	if min <= 0 {
		min = 0.55
	}
	row.DescriptionCosineBelowMin = score < min
}
