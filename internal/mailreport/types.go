package mailreport

import "github.com/behaviorengineering/taxonomy/pkg/harness"

// Source names where classify was triggered.
const (
	SourceSyncFetch = "sync_fetch"
	SourceReport    = "report"
)

// Item is one message to classify.
type Item struct {
	ObjectHash string
	Subject    string
	Sender     string
	Body       string
	Collection string
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

// MessageRow is one classified message in the artifact.
type MessageRow struct {
	ObjectHash     string       `json:"object_hash"`
	Subject        string       `json:"subject,omitempty"`
	Sender         string       `json:"sender,omitempty"`
	TermID         string       `json:"term_id,omitempty"`
	Path           []string     `json:"path,omitempty"`
	Label          string       `json:"label,omitempty"`
	Source         string       `json:"source,omitempty"`
	JudgeScore     float64      `json:"judge_score,omitempty"`
	Draft          *DraftRecord `json:"draft,omitempty"`
	CatalogApplied bool         `json:"catalog_applied"`
	Error          string       `json:"error,omitempty"`
}

// Artifact is the report-only JSON output.
type Artifact struct {
	Mode           string       `json:"mode"`
	PolypusBaseURL string       `json:"polypus_base_url"`
	JudgeModel     string       `json:"judge_model,omitempty"`
	AuthorModel    string       `json:"author_model,omitempty"`
	CatalogID      string       `json:"catalog_id"`
	GeneratedAt    string       `json:"generated_at"`
	Source         string       `json:"source"`
	Classified     int          `json:"classified"`
	Omitted        int          `json:"omitted"`
	Unresolved     int          `json:"unresolved"`
	AliasesAdded   int          `json:"aliases_added"`
	LeavesAdded    int          `json:"leaves_added"`
	ReportPath     string       `json:"report_path,omitempty"`
	Messages       []MessageRow `json:"messages"`
}

// ProgressEntry is stored per object_hash for resume.
type ProgressEntry struct {
	TermID         string   `json:"term_id,omitempty"`
	Path           []string `json:"path,omitempty"`
	Label          string   `json:"label,omitempty"`
	JudgeScore     float64  `json:"judge_score,omitempty"`
	DraftKind      string   `json:"draft_kind,omitempty"`
	CatalogApplied bool     `json:"catalog_applied"`
	Error          string   `json:"error,omitempty"`
	UpdatedAt      string   `json:"updated_at"`
}

// ProgressFile maps object_hash to progress.
type ProgressFile struct {
	Entries map[string]ProgressEntry `json:"entries"`
}

// WorldContext is the fixed host brief for taxonomy Operate.
const WorldContext = "This is a personal IMAP inbox. Report only; do not suggest deleting or moving mail. Prefer an existing catalog leaf. If the wording is a synonym of an existing leaf, draft an alias, not a new leaf. Mint a new leaf only when no existing leaf is honest."

// Seats bundles taxonomy harness seats.
type Seats struct {
	Judge  harness.Judge
	Author harness.Author
}
