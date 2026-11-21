package config

import (
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	defaultClassifyMax                = 50
	defaultCollectionINBOX            = "imap/INBOX"
	defaultAttachMinCosine            = 0.80
	defaultWalkReinforce              = 0.70
	defaultEssenceBatchSize           = 8
	maxEssenceBatchSize               = 16
	defaultDescriptionCosineMin       = 0.55
	defaultDescriptionCosineMode      = DescriptionCosineModeReport
	defaultEssenceSemanticMaxAttempts = 2
	defaultShapeMinRepeats            = 3
	defaultShapeConfirmMin            = 0.90
	defaultWorthReadMinNeighbors      = 3
	defaultWorthReadSimilarK          = 10
	maxWorthReadSimilarK              = 50
)

// DefaultShapeAllowedLeaves are last-slot KIND ids eligible for shape auto-promote.
func DefaultShapeAllowedLeaves() []string {
	return []string{"pipeline-failure", "workflow-failure", "pull-request", "release"}
}

// DescriptionCosineMode controls Essence ABOUT fit-to-email scoring and repair.
type DescriptionCosineMode string

const (
	DescriptionCosineModeOff    DescriptionCosineMode = "off"
	DescriptionCosineModeReport DescriptionCosineMode = "report"
	DescriptionCosineModeRepair DescriptionCosineMode = "repair"
)

// TaxonomyFile is the taxonomy YAML section.
type TaxonomyFile struct {
	CatalogPath                string   `yaml:"catalog_path"`
	ClassifyMax                int      `yaml:"classify_max"`
	Collections                []string `yaml:"collections"`
	AuthorOnSkip               *bool    `yaml:"author_on_skip"`
	Strategy                   string   `yaml:"strategy"`
	MinJudgeScore              float64  `yaml:"min_judge_score"`
	AttachMinCosine            float64  `yaml:"attach_min_cosine"`
	WalkReinforceMin           float64  `yaml:"walk_reinforce_min"`
	EssenceBatchSize           int      `yaml:"essence_batch_size"`
	DescriptionCosineMin       float64  `yaml:"description_cosine_min"`
	DescriptionCosineMode      string   `yaml:"description_cosine_mode"`
	EssenceSemanticMaxAttempts int      `yaml:"essence_semantic_max_attempts"`
	ShapeConfirm               *bool    `yaml:"shape_confirm"`
	ShapeMinRepeats            int      `yaml:"shape_min_repeats"`
	ShapeConfirmMin            float64  `yaml:"shape_confirm_min"`
	ShapeAllowedLeaves         []string `yaml:"shape_allowed_leaves"`
	WorthReadPrior             *bool    `yaml:"worth_read_prior"`
	WorthReadMinNeighbors      int      `yaml:"worth_read_min_neighbors"`
	WorthReadMinCosine         float64  `yaml:"worth_read_min_cosine"`
	WorthReadSimilarK          int      `yaml:"worth_read_similar_k"`
}

// TaxonomyConfig is resolved taxonomy settings.
type TaxonomyConfig struct {
	CatalogPath                string
	ClassifyMax                int
	Collections                []string
	AuthorOnSkip               bool
	Strategy                   string
	MinJudgeScore              float64
	AttachMinCosine            float64
	WalkReinforceMin           float64
	EssenceBatchSize           int
	DescriptionCosineMin       float64
	DescriptionCosineMode      DescriptionCosineMode
	EssenceSemanticMaxAttempts int
	ShapeConfirm               bool
	ShapeMinRepeats            int
	ShapeConfirmMin            float64
	ShapeAllowedLeaves         []string
	WorthReadPrior             bool
	WorthReadMinNeighbors      int
	WorthReadMinCosine         float64
	WorthReadSimilarK          int
}

// SeedCatalogPath returns the repo seed vocabulary path.
func SeedCatalogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "inbox-mail.yaml")
}

// SeedKindCatalogPath returns the repo seed inbox-kind vocabulary path.
func SeedKindCatalogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "inbox-kind.yaml")
}

// SeedTagListPath returns the repo mail tags seed list path.
func SeedTagListPath(repoRoot string) string {
	return filepath.Join(repoRoot, "config", "vocabularies", "mail-tags-seed.txt")
}

// DefaultTagRegisterPath returns the operator-generated mail tag register path.
func DefaultTagRegisterPath() (string, error) {
	const op = "config.DefaultTagRegisterPath"
	dir, err := UserConfigDir()
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "user config dir")
	}
	return filepath.Join(dir, "vocabularies", "mail-tag-register.txt"), nil
}

// DefaultTagDeprecationsPath returns the operator mail tag deprecations TOML path.
func DefaultTagDeprecationsPath() (string, error) {
	const op = "config.DefaultTagDeprecationsPath"
	dir, err := UserConfigDir()
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "user config dir")
	}
	return filepath.Join(dir, "vocabularies", "mail-tag-deprecations.toml"), nil
}

// DefaultCatalogPath returns the operator-writable catalog path.
func DefaultCatalogPath() (string, error) {
	const op = "config.DefaultCatalogPath"
	dir, err := UserConfigDir()
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "user config dir")
	}
	return filepath.Join(dir, "vocabularies", "inbox-mail.yaml"), nil
}

// DefaultKindCatalogPath returns the operator-writable inbox-kind catalog path.
func DefaultKindCatalogPath() (string, error) {
	const op = "config.DefaultKindCatalogPath"
	dir, err := UserConfigDir()
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "user config dir")
	}
	return filepath.Join(dir, "vocabularies", "inbox-kind.yaml"), nil
}

func resolveTaxonomy(file TaxonomyFile) (TaxonomyConfig, error) {
	const op = "config.resolveTaxonomy"
	strategy, err := NormalizeTaxonomyStrategy(file.Strategy)
	if err != nil {
		return TaxonomyConfig{}, err
	}
	path, err := resolveTaxonomyCatalogPath(op, strategy, file.CatalogPath)
	if err != nil {
		return TaxonomyConfig{}, err
	}
	base := resolveTaxonomyBase(file)
	attachMin, walkMin, err := resolveTaxonomyAttachLimits(file)
	if err != nil {
		return TaxonomyConfig{}, err
	}
	essence, err := resolveTaxonomyEssence(file)
	if err != nil {
		return TaxonomyConfig{}, err
	}
	shape := resolveTaxonomyShape(file)
	worth, err := resolveTaxonomyWorthRead(file)
	if err != nil {
		return TaxonomyConfig{}, err
	}
	return TaxonomyConfig{
		CatalogPath:                path,
		ClassifyMax:                base.classifyMax,
		Collections:                base.collections,
		AuthorOnSkip:               base.authorOnSkip,
		Strategy:                   strategy,
		MinJudgeScore:              file.MinJudgeScore,
		AttachMinCosine:            attachMin,
		WalkReinforceMin:           walkMin,
		EssenceBatchSize:           essence.batchSize,
		DescriptionCosineMin:       essence.descMin,
		DescriptionCosineMode:      essence.descMode,
		EssenceSemanticMaxAttempts: essence.semanticAttempts,
		ShapeConfirm:               shape.confirm,
		ShapeMinRepeats:            shape.minRepeats,
		ShapeConfirmMin:            shape.confirmMin,
		ShapeAllowedLeaves:         shape.allowedLeaves,
		WorthReadPrior:             worth.prior,
		WorthReadMinNeighbors:      worth.minNeighbors,
		WorthReadMinCosine:         worth.minCosine,
		WorthReadSimilarK:          worth.similarK,
	}, nil
}

func resolveTaxonomyCatalogPath(op string, strategy string, catalogPath string) (string, error) {
	path := strings.TrimSpace(catalogPath)
	if path != "" {
		return path, nil
	}
	var err error
	if IsAttachStrategy(strategy) {
		path, err = DefaultKindCatalogPath()
	} else {
		path, err = DefaultCatalogPath()
	}
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "default catalog path")
	}
	return path, nil
}

type taxonomyBaseDefaults struct {
	classifyMax  int
	collections  []string
	authorOnSkip bool
}

func resolveTaxonomyBase(file TaxonomyFile) taxonomyBaseDefaults {
	max := file.ClassifyMax
	if max <= 0 {
		max = defaultClassifyMax
	}
	collections := uniqueNonEmptyStrings(file.Collections)
	if len(collections) == 0 {
		collections = []string{defaultCollectionINBOX}
	}
	authorOnSkip := false
	if file.AuthorOnSkip != nil {
		authorOnSkip = *file.AuthorOnSkip
	}
	return taxonomyBaseDefaults{classifyMax: max, collections: collections, authorOnSkip: authorOnSkip}
}

func resolveTaxonomyAttachLimits(file TaxonomyFile) (attachMin, walkMin float64, err error) {
	const op = "config.resolveTaxonomy"
	attachMin = file.AttachMinCosine
	if attachMin == 0 {
		attachMin = defaultAttachMinCosine
	}
	walkMin = file.WalkReinforceMin
	if walkMin == 0 {
		walkMin = defaultWalkReinforce
	}
	if walkMin > attachMin {
		return 0, 0, sirerr.New(sirerr.CodeInvalid, op, "walk_reinforce_min must be <= attach_min_cosine")
	}
	return attachMin, walkMin, nil
}

type taxonomyEssenceDefaults struct {
	batchSize        int
	descMode         DescriptionCosineMode
	descMin          float64
	semanticAttempts int
}

func resolveTaxonomyEssence(file TaxonomyFile) (taxonomyEssenceDefaults, error) {
	essenceBatch := file.EssenceBatchSize
	if essenceBatch <= 0 {
		essenceBatch = defaultEssenceBatchSize
	}
	if essenceBatch > maxEssenceBatchSize {
		essenceBatch = maxEssenceBatchSize
	}
	descMode, err := NormalizeDescriptionCosineMode(file.DescriptionCosineMode)
	if err != nil {
		return taxonomyEssenceDefaults{}, err
	}
	descMin := file.DescriptionCosineMin
	if descMin == 0 {
		descMin = defaultDescriptionCosineMin
	}
	semanticAttempts := file.EssenceSemanticMaxAttempts
	if semanticAttempts <= 0 {
		semanticAttempts = defaultEssenceSemanticMaxAttempts
	}
	return taxonomyEssenceDefaults{
		batchSize: essenceBatch, descMode: descMode, descMin: descMin, semanticAttempts: semanticAttempts,
	}, nil
}

type taxonomyShapeDefaults struct {
	confirm       bool
	minRepeats    int
	confirmMin    float64
	allowedLeaves []string
}

func resolveTaxonomyShape(file TaxonomyFile) taxonomyShapeDefaults {
	shapeConfirm := true
	if file.ShapeConfirm != nil {
		shapeConfirm = *file.ShapeConfirm
	}
	shapeMinRepeats := file.ShapeMinRepeats
	if shapeMinRepeats <= 0 {
		shapeMinRepeats = defaultShapeMinRepeats
	}
	shapeConfirmMin := file.ShapeConfirmMin
	if shapeConfirmMin <= 0 {
		shapeConfirmMin = defaultShapeConfirmMin
	}
	shapeLeaves := uniqueNonEmptyStrings(file.ShapeAllowedLeaves)
	if len(shapeLeaves) == 0 {
		shapeLeaves = DefaultShapeAllowedLeaves()
	}
	return taxonomyShapeDefaults{
		confirm: shapeConfirm, minRepeats: shapeMinRepeats, confirmMin: shapeConfirmMin, allowedLeaves: shapeLeaves,
	}
}

type taxonomyWorthReadDefaults struct {
	prior        bool
	minNeighbors int
	minCosine    float64
	similarK     int
}

func resolveTaxonomyWorthRead(file TaxonomyFile) (taxonomyWorthReadDefaults, error) {
	const op = "config.resolveTaxonomy"
	worthReadPrior := true
	if file.WorthReadPrior != nil {
		worthReadPrior = *file.WorthReadPrior
	}
	worthNeighbors := file.WorthReadMinNeighbors
	if worthNeighbors < 1 {
		worthNeighbors = defaultWorthReadMinNeighbors
	}
	worthCosine := file.WorthReadMinCosine
	if worthCosine < 0 {
		return taxonomyWorthReadDefaults{}, sirerr.New(sirerr.CodeInvalid, op, "worth_read_min_cosine must be >= 0")
	}
	worthK := file.WorthReadSimilarK
	if worthK <= 0 {
		worthK = defaultWorthReadSimilarK
	}
	if worthK > maxWorthReadSimilarK {
		worthK = maxWorthReadSimilarK
	}
	return taxonomyWorthReadDefaults{
		prior: worthReadPrior, minNeighbors: worthNeighbors, minCosine: worthCosine, similarK: worthK,
	}, nil
}

// NormalizeDescriptionCosineMode parses taxonomy description_cosine_mode.
func NormalizeDescriptionCosineMode(raw string) (DescriptionCosineMode, error) {
	const op = "config.NormalizeDescriptionCosineMode"
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return defaultDescriptionCosineMode, nil
	}
	switch DescriptionCosineMode(s) {
	case DescriptionCosineModeOff, DescriptionCosineModeReport, DescriptionCosineModeRepair:
		return DescriptionCosineMode(s), nil
	default:
		return "", sirerr.New(sirerr.CodeInvalid, op, "unknown description_cosine_mode")
	}
}

func uniqueNonEmptyStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
