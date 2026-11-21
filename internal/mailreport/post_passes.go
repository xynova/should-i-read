package mailreport

import (
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/tags"

	"github.com/xynova/should-i-read/internal/hoststore"
)

// PostPasses is tag, cluster, attention, and worth-read after Operate persist.
// Runner embeds it so Classify still sees store and clock, and Cluster still promotes.
type PostPasses struct {
	store                 *hoststore.Store
	seats                 Seats
	max                   int
	aiHops                int
	judgeModel            string
	embedModel            string
	attachMinCosine       float64
	now                   time.Time
	tldrBatch             ChatLineRunner
	tldrModel             string
	clusterMinCosine      float64
	quarantineTerms       []string
	attention             AttentionClient
	attentionTrueMin      float64
	tagCatalog            *tags.Catalog
	tagCurator            *TagCurator
	shapeConfirm          bool
	shapeMinRepeats       int
	shapeConfirmMin       float64
	shapeAllowedLeaves    []string
	subjectTemplater      *SubjectTemplater
	worthReadPrior        bool
	worthReadMinNeighbors int
	worthReadMinCosine    float64
	worthReadSimilarK     int
}
