package mailreport

import (
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimdir"
)

// Source names where classify was triggered.
const (
	SourceSyncFetch = "sync_fetch"
	SourceReport    = "report"
	SourceBench     = "bench"
)

// Item is one message to classify.
type Item struct {
	ObjectHash string
	Subject    string
	Sender     string
	Body       string
	Collection string
	Headers    pimdir.MailHeaders
}

// DraftRecord captures an accepted or rejected draft.
type DraftRecord struct {
	Kind        string `json:"kind,omitempty"`
	ID          string `json:"id,omitempty"`
	Parent      string `json:"parent,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	LeafID      string `json:"leaf_id,omitempty"`
	Alias       string `json:"alias,omitempty"`
}

const (
	SuggestedReview              = hoststore.SuggestedReview
	SuggestedQuarantineCandidate = hoststore.SuggestedQuarantineCandidate
)

// Attention is report-only urgency and action signals from Jev noul scores.
type Attention struct {
	Urgency              float64 `json:"urgency"`
	NeedsReply           bool    `json:"needs_reply"`
	InvolvesDecision     bool    `json:"involves_decision"`
	NeedsReplyNoul       float64 `json:"needs_reply_noul,omitempty"`
	InvolvesDecisionNoul float64 `json:"involves_decision_noul,omitempty"`
}

// ClusterRow is one cluster in the report artifact.
type ClusterRow struct {
	ID              string     `json:"id"`
	TermID          string     `json:"term_id,omitempty"`
	SenderDomain    string     `json:"sender_domain,omitempty"`
	Size            int        `json:"size"`
	TLDR            string     `json:"tldr,omitempty"`
	SuggestedAction string     `json:"suggested_action"`
	Attention       *Attention `json:"attention,omitempty"`
	MemberHashes    []string   `json:"member_hashes"`
	Fingerprint     string     `json:"fingerprint,omitempty"`
	TLDRError       string     `json:"tldr_error,omitempty"`
}

// MessageRow is one classified message in the artifact.
type MessageRow struct {
	ObjectHash                string       `json:"object_hash"`
	Subject                   string       `json:"subject,omitempty"`
	Sender                    string       `json:"sender,omitempty"`
	TermID                    string       `json:"term_id,omitempty"`
	Path                      []string     `json:"path,omitempty"`
	Label                     string       `json:"label,omitempty"`
	Source                    string       `json:"source,omitempty"`
	Strategy                  string       `json:"strategy,omitempty"`
	Kind                      string       `json:"kind,omitempty"`
	About                     string       `json:"about,omitempty"`
	Shape                     string       `json:"shape,omitempty"`
	Tags                      []string     `json:"tags,omitempty"`
	Cosine                    float64      `json:"cosine,omitempty"`
	DescriptionCosine         float64      `json:"description_cosine,omitempty"`
	DescriptionCosineBelowMin bool         `json:"description_cosine_below_min,omitempty"`
	CanonicalTermID           string       `json:"canonical_term_id,omitempty"`
	Reinforced                bool         `json:"reinforced,omitempty"`
	JudgeScore                float64      `json:"judge_score,omitempty"`
	Draft                     *DraftRecord `json:"draft,omitempty"`
	CatalogApplied            bool         `json:"catalog_applied"`
	Error                     string       `json:"error,omitempty"`
	EmbedPersist              string       `json:"embed_persist,omitempty"`
	SenderTermID              string       `json:"sender_term_id,omitempty"`
	SenderLabel               string       `json:"sender_label,omitempty"`
	SenderMapsTo              string       `json:"sender_maps_to,omitempty"`
	ClusterID                 string       `json:"cluster_id,omitempty"`
	Attention                 *Attention   `json:"attention,omitempty"`
	AttentionError            string       `json:"attention_error,omitempty"`
}

// Artifact is the report-only JSON output.
type Artifact struct {
	Mode                 string            `json:"mode"`
	PolypusBaseURL       string            `json:"polypus_base_url"`
	JudgeModel           string            `json:"judge_model,omitempty"`
	AuthorModel          string            `json:"author_model,omitempty"`
	EmbedModel           string            `json:"embed_model,omitempty"`
	AttachMinCosine      float64           `json:"attach_min_cosine,omitempty"`
	WalkReinforceMin     float64           `json:"walk_reinforce_min,omitempty"`
	CatalogID            string            `json:"catalog_id"`
	SendersCatalogID     string            `json:"senders_catalog_id,omitempty"`
	GeneratedAt          string            `json:"generated_at"`
	WindowFrom           string            `json:"window_from,omitempty"`
	WindowTo             string            `json:"window_to,omitempty"`
	Source               string            `json:"source"`
	Classified           int               `json:"classified"`
	NewlyClassified      int               `json:"newly_classified,omitempty"`
	Omitted              int               `json:"omitted"`
	Unresolved           int               `json:"unresolved"`
	AliasesAdded         int               `json:"aliases_added"`
	LeavesAdded          int               `json:"leaves_added"`
	ReportPath           string            `json:"report_path,omitempty"`
	TldrModel            string            `json:"tldr_model,omitempty"`
	EssenceModel         string            `json:"essence_model,omitempty"`
	EssenceEscalateModel string            `json:"essence_escalate_model,omitempty"`
	AttentionTrueMin     float64           `json:"attention_true_min,omitempty"`
	AttentionScored      int               `json:"attention_scored,omitempty"`
	AttentionOmitted     int               `json:"attention_omitted,omitempty"`
	EssenceDumpDir       string            `json:"essence_dump_dir,omitempty"`
	TagRegisterPath      string            `json:"tag_register_path,omitempty"`
	TagRegisterError     string            `json:"tag_register_error,omitempty"`
	TagCurateError       string            `json:"tag_curate_error,omitempty"`
	TagCurateDropped     []string          `json:"tag_curate_dropped,omitempty"`
	TagCurateKept        []string          `json:"tag_curate_kept,omitempty"`
	TagCurateMerged      map[string]string `json:"tag_curate_merged,omitempty"`
	TagCurateJevSkipped  bool              `json:"tag_curate_jev_skipped,omitempty"`
	TagPassSkipped       bool              `json:"tag_pass_skipped,omitempty"`
	ShapeConfirmed       int               `json:"shape_confirmed,omitempty"`
	ShapesPromoted       int               `json:"shapes_promoted,omitempty"`
	ShapeStoreError      string            `json:"shape_store_error,omitempty"`
	IncompleteError      string            `json:"incomplete_error,omitempty"`
	Messages             []MessageRow      `json:"messages"`
	Clusters             []ClusterRow      `json:"clusters,omitempty"`
}

// ProgressEntry is stored per object_hash for resume.
type ProgressEntry struct {
	TermID          string   `json:"term_id,omitempty"`
	Path            []string `json:"path,omitempty"`
	Label           string   `json:"label,omitempty"`
	Source          string   `json:"source,omitempty"`
	Strategy        string   `json:"strategy,omitempty"`
	Kind            string   `json:"kind,omitempty"`
	About           string   `json:"about,omitempty"`
	Shape           string   `json:"shape,omitempty"`
	Cosine          float64  `json:"cosine,omitempty"`
	CanonicalTermID string   `json:"canonical_term_id,omitempty"`
	Reinforced      bool     `json:"reinforced,omitempty"`
	JudgeScore      float64  `json:"judge_score,omitempty"`
	DraftKind       string   `json:"draft_kind,omitempty"`
	CatalogApplied  bool     `json:"catalog_applied"`
	Error           string   `json:"error,omitempty"`
	SenderTermID    string   `json:"sender_term_id,omitempty"`
	SenderLabel     string   `json:"sender_label,omitempty"`
	SenderMapsTo    string   `json:"sender_maps_to,omitempty"`
	UpdatedAt       string   `json:"updated_at"`
}

// ProgressFile maps object_hash to progress.
type ProgressFile struct {
	Entries map[string]ProgressEntry `json:"entries"`
}

// WorldContext is the fixed host brief for taxonomy Operate.
const WorldContext = "This is a personal IMAP inbox. Report only; do not suggest deleting or moving mail. Prefer an existing catalog leaf. If the wording is a synonym of an existing leaf, draft an alias, not a new leaf. Mint a new leaf only when no existing leaf is honest."

// Seats bundles taxonomy harness seats.
type Seats struct {
	Judge    harness.Judge
	Author   harness.Author
	Essencer harness.Essencer
	Embedder harness.Embedder
}
