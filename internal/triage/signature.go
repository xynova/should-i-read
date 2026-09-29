// Package triage holds report-only classification contracts (strop + Polypus later).
package triage

// Category is a coarse mail class for digest grouping.
type Category string

const (
	CategoryActionable    Category = "actionable"
	CategoryPersonal      Category = "personal"
	CategoryNewsletter    Category = "newsletter"
	CategoryNotification  Category = "notification"
	CategoryColdOutreach  Category = "cold_outreach"
	CategorySpam          Category = "spam"
	CategoryOther         Category = "other"
)

// Decision is one strop structured output per message (future).
type Decision struct {
	ShouldRead       bool     `json:"should_read"`
	Category         Category `json:"category"`
	UrgencyScore     int      `json:"urgency_score"`
	OneLineRationale string   `json:"one_line_rationale"`
}

// DigestArtifact is the daily report-only JSON shape.
type DigestArtifact struct {
	Mode           string        `json:"mode"`
	PolypusBaseURL string        `json:"polypus_base_url"`
	GeneratedAt    string        `json:"generated_at"`
	MustRead       []DigestEntry `json:"must_read"`
	FYI            []DigestEntry `json:"fyi"`
	Unwanted       []DigestEntry `json:"unwanted_clusters"`
}

// DigestEntry links a store row to a triage decision.
type DigestEntry struct {
	Seq      int64    `json:"seq"`
	Subject  string   `json:"subject"`
	Sender   string   `json:"sender,omitempty"`
	Decision Decision `json:"decision"`
	Excerpt  string   `json:"excerpt,omitempty"`
}
