package mailoverview

// SchemaVersion of the JSON sidecar and HTML data block.
const SchemaVersion = 1

// Input for building an overview.
type Input struct {
	SinceMonth string
	UntilMonth string
	MaxTable   int
}

// Model is the full report payload, written to JSON and embedded in HTML.
type Model struct {
	SchemaVersion int            `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	LedgerPath    string         `json:"ledger_path"`
	MailStorePath string         `json:"mail_store_path"`
	CLIVersion    string         `json:"cli_version"`
	Provenance    Provenance     `json:"provenance"`
	Months        []MonthSection `json:"months"`
	All           MonthSection   `json:"all"`
	Shapes        []ShapeInfo    `json:"shapes"`
	ErrorRows     []ErrorRowInfo `json:"error_rows"`
}

// Provenance records paths and counts for the footer.
type Provenance struct {
	CompleteRows int    `json:"complete_rows"`
	MailMatched  int    `json:"mail_matched"`
	MailMissing  int    `json:"mail_missing"`
	GitRevision  string `json:"git_revision,omitempty"`
}

// MonthSection is one tab (a calendar month or All).
type MonthSection struct {
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	Summary   SummaryStats   `json:"summary"`
	Attention AttentionBlock `json:"attention"`
	Themes    []ThemeCount   `json:"themes"`
	Heatmap   HeatmapData    `json:"heatmap"`
	Fluff     FluffBlock     `json:"fluff"`
	KindTree  KindTreeBlock  `json:"kind_tree"`
	Pipeline  PipelineBlock  `json:"pipeline"`
	Trust     TrustBlock     `json:"trust"`
	Messages  []MessageRow   `json:"messages"`
}

// SummaryStats appears in the summary strip.
type SummaryStats struct {
	Classified      int      `json:"classified"`
	JevSkips        int      `json:"jev_skips"`
	EssenceChats    int      `json:"essence_chats"`
	EmbedsSaved     int      `json:"embeds_saved"`
	NeedsReply      int      `json:"needs_reply"`
	FluffMinutes    float64  `json:"fluff_minutes"`
	DeltaClassified int      `json:"delta_classified,omitempty"`
	Sparkline       []int    `json:"sparkline,omitempty"`
	SparkMonths     []string `json:"spark_months,omitempty"`
}

// AttentionBlock holds attention aggregates and lists.
type AttentionBlock struct {
	Quiet        int           `json:"quiet"`
	UrgentOnly   int           `json:"urgent_only"`
	NeedsReply   int           `json:"needs_reply"`
	Decision     int           `json:"decision"`
	ReplyList    []MessageLine `json:"reply_list"`
	DecisionList []MessageLine `json:"decision_list"`
	UrgencyHist  Histogram     `json:"urgency_hist"`
	Donut        []DonutSlice  `json:"donut"`
}

// Histogram has fixed buckets for urgency.
type Histogram struct {
	Buckets   []float64 `json:"buckets"`
	Counts    []int     `json:"counts"`
	Threshold float64   `json:"threshold"`
}

// DonutSlice is one arc segment.
type DonutSlice struct {
	Label    string  `json:"label"`
	Count    int     `json:"count"`
	Fraction float64 `json:"fraction,omitempty"`
}

// ThemeCount is one last-slot theme bucket.
type ThemeCount struct {
	Slot  string `json:"slot"`
	Count int    `json:"count"`
}

// HeatmapData is week labels x theme slots.
type HeatmapData struct {
	WeekLabels  []string `json:"week_labels"`
	ThemeLabels []string `json:"theme_labels"`
	Cells       [][]int  `json:"cells"`
	// CellHashes is comma-joined object hashes, parallel to Cells.
	CellHashes [][]string `json:"cell_hashes,omitempty"`
	// ThemeHashes is comma-joined object hashes per theme row, all weeks.
	ThemeHashes []string `json:"theme_hashes,omitempty"`
}

// KindTreeBlock groups first-segment shared paths vs one-off outliers.
type KindTreeBlock struct {
	Trunk    []KindTrunkNode   `json:"trunk"`
	Outliers []KindOutlierLeaf `json:"outliers"`
	Paths    []KindPathRow     `json:"paths"`
}

// KindTrunkNode is a shared first KIND segment.
type KindTrunkNode struct {
	FirstSlot string `json:"first_slot"`
	Count     int    `json:"count"`
}

// KindOutlierLeaf is a one-off first segment in this view.
type KindOutlierLeaf struct {
	FirstSlot string `json:"first_slot"`
	Kind      string `json:"kind"`
	Why       string `json:"why"`
}

// KindPathRow is one full KIND path with count and optional note.
type KindPathRow struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
	Why   string `json:"why,omitempty"`
	// Hashes are object_hash values whose KIND is this path.
	Hashes []string `json:"hashes,omitempty"`
}

// FluffBlock is fluff tax by week and sender rollup.
type FluffBlock struct {
	ByWeek        []WeekFluff   `json:"by_week"`
	TopSenders    []SenderFluff `json:"top_senders"`
	DeleteMinutes float64       `json:"delete_minutes"`
	KeepMinutes   float64       `json:"keep_minutes"`
}

// WeekFluff is one week bucket.
type WeekFluff struct {
	WeekKey string  `json:"week_key"`
	Label   string  `json:"label"`
	Minutes float64 `json:"minutes"`
}

// SenderFluff rolls up by sender display.
type SenderFluff struct {
	Sender   string   `json:"sender"`
	Minutes  float64  `json:"minutes"`
	Messages int      `json:"messages"`
	Hashes   []string `json:"hashes,omitempty"`
}

// HashAttr joins Hashes for a data-hashes attribute.
func (s SenderFluff) HashAttr() string {
	return JoinHashes(s.Hashes)
}

// PipelineBlock describes hops and templates.
type PipelineBlock struct {
	HopFlow        HopFlow        `json:"hop_flow"`
	SkipRateWeek   []WeekPoint    `json:"skip_rate_week"`
	ModelsClassify map[string]int `json:"models_classify"`
	ModelsEmbed    map[string]int `json:"models_embed"`
	PromptHashes   map[string]int `json:"prompt_hashes"`
}

// HopFlow is a small flow with edge counts.
type HopFlow struct {
	Nodes []string  `json:"nodes"`
	Edges []HopEdge `json:"edges"`
}

// HopEdge is a directed edge with a count.
type HopEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

// WeekPoint is one week with a rate in 0..1.
type WeekPoint struct {
	WeekKey string  `json:"week_key"`
	Label   string  `json:"label"`
	Rate    float64 `json:"rate"`
	Skips   int     `json:"skips"`
	Total   int     `json:"total"`
}

// TrustBlock has confidence stats.
type TrustBlock struct {
	CosineHist   Histogram     `json:"cosine_hist"`
	JudgeHist    Histogram     `json:"judge_hist"`
	LowConf      []MessageLine `json:"low_conf"`
	NotAttention int           `json:"not_attention_scored"`
	ErrorGroups  []ErrorGroup  `json:"error_groups,omitempty"`
}

// ErrorGroup rolls up classify_state errors by message text.
type ErrorGroup struct {
	Error         string `json:"error"`
	Rows          int    `json:"rows"`
	TotalAttempts int    `json:"total_attempts"`
}

// MessageLine is a compact row for lists.
type MessageLine struct {
	ObjectHash string  `json:"object_hash"`
	Date       string  `json:"date"`
	Sender     string  `json:"sender"`
	SenderName string  `json:"sender_name,omitempty"`
	Subject    string  `json:"subject"`
	Theme      string  `json:"theme"`
	Kind       string  `json:"kind"`
	Urgency    float64 `json:"urgency"`
	NeedsReply float64 `json:"needs_reply"`
	Decision   float64 `json:"decision"`
	Cosine     float64 `json:"cosine"`
	JudgeScore float64 `json:"judge_score"`
}

// MessageRow is the full table row.
type MessageRow struct {
	MessageLine
	Strategy     string `json:"strategy"`
	WeekKey      string `json:"week_key"`
	MonthKey     string `json:"month_key"`
	OutlierFirst bool   `json:"outlier_first"`
	Why          string `json:"why,omitempty"`
}

// ShapeInfo is one mail_shapes row for the template table.
type ShapeInfo struct {
	Fingerprint     string `json:"fingerprint"`
	SenderDomain    string `json:"sender_domain"`
	SubjectTemplate string `json:"subject_template"`
	LastSlot        string `json:"last_slot"`
	Status          string `json:"status"`
	HitCount        int    `json:"hit_count"`
	ConfirmCount    int    `json:"confirm_count"`
	RejectCount     int    `json:"reject_count"`
}

// ErrorRowInfo is one classify error.
type ErrorRowInfo struct {
	ObjectHash   string `json:"object_hash"`
	Error        string `json:"error"`
	AttemptCount int    `json:"attempt_count"`
	UpdatedAt    string `json:"updated_at"`
}
