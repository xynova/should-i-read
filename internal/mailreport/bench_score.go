package mailreport

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// BenchLabel is one human-scored message in the benchmark subset.
type BenchLabel struct {
	ObjectHash    string   `json:"object_hash"`
	SubjectHint   string   `json:"subject_hint"`
	ExpectedLeaf  string   `json:"expected_leaf"`
	RequiredTerms []string `json:"required_terms"`
}

// BenchLabelsFile is the on-disk labels schema.
type BenchLabelsFile struct {
	Revision string       `json:"revision"`
	Labels   []BenchLabel `json:"labels"`
}

// BenchScore is the per-variant scorecard JSON.
type BenchScore struct {
	Variant              string             `json:"variant"`
	InputRev             string             `json:"input_rev"`
	PromptHash           string             `json:"prompt_hash"`
	EssenceModel         string             `json:"essence_model,omitempty"`
	EssenceEscalateModel string             `json:"essence_escalate_model,omitempty"`
	JudgeModel           string             `json:"judge_model,omitempty"`
	EmbedModel           string             `json:"embed_model,omitempty"`
	DescriptionCosineMin float64            `json:"description_cosine_min,omitempty"`
	Classified           int                `json:"classified"`
	Failed               int                `json:"failed"`
	Omitted              int                `json:"omitted"`
	Unresolved           int                `json:"unresolved"`
	CauseCounts          map[string]int     `json:"cause_counts"`
	CosineMean           float64            `json:"cosine_mean"`
	CosineP50            float64            `json:"cosine_p50"`
	CosineBelowMinShare  float64            `json:"cosine_below_min_share"`
	LeafAccuracy         float64            `json:"leaf_accuracy"`
	MeanTermRecall       float64            `json:"mean_term_recall"`
	LabeledScored        int                `json:"labeled_scored"`
	ChunkRuntimesS       map[string]float64 `json:"chunk_runtimes_s"`
	WallS                float64            `json:"wall_s"`
}

// LoadBenchLabels reads labels JSON and validates revision when expectedRev is non-empty.
func LoadBenchLabels(path, expectedRev string) (BenchLabelsFile, error) {
	const op = "mailreport.LoadBenchLabels"
	raw, err := os.ReadFile(path)
	if err != nil {
		return BenchLabelsFile{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "read labels")
	}
	var file BenchLabelsFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return BenchLabelsFile{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode labels")
	}
	if expectedRev != "" && strings.TrimSpace(file.Revision) != strings.TrimSpace(expectedRev) {
		return BenchLabelsFile{}, sirerr.New(sirerr.CodeInvalid, op, "labels revision mismatch").
			With("expected", expectedRev).With("got", file.Revision)
	}
	return file, nil
}

// ScoreBenchArtifact builds a scorecard from a merged classify artifact.
func ScoreBenchArtifact(art Artifact, variant, inputRev string, labels BenchLabelsFile, promptHash string, descMin float64, chunkRuntimes map[string]time.Duration, wall time.Duration) BenchScore {
	byHash := map[string]MessageRow{}
	for _, m := range art.Messages {
		byHash[strings.TrimSpace(m.ObjectHash)] = m
	}
	causeCounts := map[string]int{}
	failed := 0
	for _, m := range art.Messages {
		if strings.TrimSpace(m.Error) == "" {
			continue
		}
		failed++
		cause := ClassifyFailureCause(m.Error)
		if cause == "" {
			cause = "other"
		}
		causeCounts[cause]++
	}
	cosines := make([]float64, 0, len(art.Messages))
	below := 0
	for _, m := range art.Messages {
		if m.DescriptionCosine == 0 && !m.DescriptionCosineBelowMin {
			continue
		}
		cosines = append(cosines, m.DescriptionCosine)
		if descMin > 0 && m.DescriptionCosine < descMin {
			below++
		}
	}
	mean, p50 := cosineStats(cosines)
	belowShare := 0.0
	if len(cosines) > 0 {
		belowShare = float64(below) / float64(len(cosines))
	}
	leafHits := 0
	termSum := 0.0
	labeled := len(labels.Labels)
	for _, lb := range labels.Labels {
		row, ok := byHash[strings.TrimSpace(lb.ObjectHash)]
		if !ok {
			continue
		}
		if leafMatches(row.Kind, lb.ExpectedLeaf) {
			leafHits++
		}
		termSum += termRecall(row.About, lb.RequiredTerms)
	}
	leafAcc := 0.0
	meanRecall := 0.0
	if labeled > 0 {
		leafAcc = float64(leafHits) / float64(labeled)
		meanRecall = termSum / float64(labeled)
	}
	runtimeS := map[string]float64{}
	for k, d := range chunkRuntimes {
		runtimeS[k] = d.Seconds()
	}
	return BenchScore{
		Variant:              variant,
		InputRev:             inputRev,
		PromptHash:           promptHash,
		EssenceModel:         art.EssenceModel,
		EssenceEscalateModel: art.EssenceEscalateModel,
		JudgeModel:           art.JudgeModel,
		EmbedModel:           art.EmbedModel,
		DescriptionCosineMin: descMin,
		Classified:           art.Classified,
		Failed:               failed,
		Omitted:              art.Omitted,
		Unresolved:           art.Unresolved,
		CauseCounts:          causeCounts,
		CosineMean:           mean,
		CosineP50:            p50,
		CosineBelowMinShare:  belowShare,
		LeafAccuracy:         leafAcc,
		MeanTermRecall:       meanRecall,
		LabeledScored:        labeled,
		ChunkRuntimesS:       runtimeS,
		WallS:                wall.Seconds(),
	}
}

func leafMatches(kind, expectedLeaf string) bool {
	expectedLeaf = strings.TrimSpace(expectedLeaf)
	if expectedLeaf == "" {
		return false
	}
	segs := catalog.ParseKindSegments(strings.TrimSpace(kind))
	if len(segs) == 0 {
		return false
	}
	return strings.TrimSpace(segs[len(segs)-1]) == expectedLeaf
}

func termRecall(about string, terms []string) float64 {
	if len(terms) == 0 {
		return 0
	}
	lower := strings.ToLower(about)
	matched := 0
	for _, t := range terms {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" {
			continue
		}
		if strings.Contains(lower, t) {
			matched++
		}
	}
	return float64(matched) / float64(len(terms))
}

func cosineStats(vals []float64) (mean, p50 float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean = sum / float64(len(vals))
	sorted := append([]float64{}, vals...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		p50 = sorted[mid]
	} else {
		p50 = (sorted[mid-1] + sorted[mid]) / 2
	}
	return mean, p50
}

// WriteBenchScore writes a scorecard JSON file.
func WriteBenchScore(path string, score BenchScore) error {
	const op = "mailreport.WriteBenchScore"
	raw, err := json.MarshalIndent(score, "", "  ")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode score")
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write score").With("path", path)
	}
	return nil
}
