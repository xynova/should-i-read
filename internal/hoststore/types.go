package hoststore

import "time"

// StateRow is a persisted classify outcome.
type StateRow struct {
	ObjectHash           string
	Status               string
	TermID               string
	Path                 []string
	Label                string
	Source               string
	Strategy             string
	Kind                 string
	About                string
	Shape                string
	Cosine               float64
	CanonicalTermID      string
	Reinforced           bool
	JudgeScore           float64
	DraftKind            string
	CatalogApplied       bool
	SenderTermID         string
	SenderLabel          string
	SenderMapsTo         string
	Error                string
	CatalogID            string
	ClassifyModel        string
	EmbedModel           string
	ClaimToken           string
	LeaseUntil           string
	AttemptCount         int
	Urgency              float64
	NeedsReplyNoul       float64
	InvolvesDecisionNoul float64
	AttentionScored      bool
	AttentionError       string
	UpdatedAt            string
}

// Counts summarizes classify_state rows.
type Counts struct {
	Complete int
	Error    int
	Running  int
}

// Neighbor is a similar-mail hit.
type Neighbor struct {
	ObjectHash string
	Cosine     float64
}

// ClusterRecord is a persisted cluster with members.
type ClusterRecord struct {
	ClusterID        string
	Fingerprint      string
	TermID           string
	SenderDomain     string
	Size             int
	TLDR             string
	SuggestedAction  string
	TLDRModel        string
	Urgency          float64
	NeedsReply       bool
	InvolvesDecision bool
	AttentionScored  bool
	MemberHashes     []string
	UpdatedAt        string
}

// ProgressEntry mirrors mailreport progress JSON for import.
type ProgressEntry struct {
	TermID          string
	Path            []string
	Label           string
	Source          string
	Strategy        string
	Kind            string
	About           string
	Shape           string
	Cosine          float64
	CanonicalTermID string
	Reinforced      bool
	JudgeScore      float64
	DraftKind       string
	CatalogApplied  bool
	Error           string
	SenderTermID    string
	SenderLabel     string
	SenderMapsTo    string
	UpdatedAt       string
}

// StoreConfig configures CreateStore.
type StoreConfig struct {
	Path string
	Now  time.Time
}
