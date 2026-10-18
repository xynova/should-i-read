package mailreport

import (
	"context"
	"strings"
	"time"

	"github.com/behaviorengineering/strop/pkg/jev"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"github.com/behaviorengineering/taxonomy/pkg/tags"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	tagCurateStoreTimeout = 30 * time.Second
	tagCurateJevTimeout   = 2 * time.Minute
	defaultTagCurateMin   = 0.80
	tagCurateWorld        = "Report only. Do not delete, move, or mark mail. Mail tags are a small flat vocabulary used to filter and group classified messages by reusable mail form (receipt, newsletter, legal-notice, calendar-invite, time-critical). Tags are not KIND topic paths, not email anatomy (subject, body, sender, message), and not layout or geometry."
	tagDropDescription    = "The token is not a reusable mail-form tag. Drop it if it names email anatomy, layout or formatting, a KIND topic or sender path, or a generic adjective that would apply to most mail."
	tagKeepDescription    = "The token is a distinct functional mail genre the operator could filter or group on, not a topic, layout word, or email-anatomy word."
	tagKeepOwnDescription = "Keep as its own mail-form tag: a distinct reusable genre not covered by any listed seed."
	tagTokenUseLine       = "Operator use is to filter and group mail by form, not by topic or layout."
)

// TagCurateTimeout is the Judge hop deadline for mail tag curation.
const TagCurateTimeout = tagCurateJevTimeout

// TagPassTimeout is the parent budget for a full tag pass (two store phases plus Judge).
const TagPassTimeout = tagCurateJevTimeout + tagCurateStoreTimeout*2

// TagCuratorConfig configures ledger tag unify and optional Jev curation.
type TagCuratorConfig struct {
	Store      *hoststore.Store
	Catalog    *tags.Catalog
	Client     *polypus.Client
	JudgeModel string
	MinScore   float64
	Judge      harness.Judge
}

// TagCurator rewrites classify_state.shape and optional Jev cleanup.
type TagCurator struct {
	inner *tags.Curator
}

// CurateResult summarizes a tag curation run.
type CurateResult = tags.Result

// CreateTagCurator builds a curator after nil checks on store and catalog.
func CreateTagCurator(cfg TagCuratorConfig) (*TagCurator, error) {
	const op = "mailreport.CreateTagCurator"
	if cfg.Store == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if cfg.Catalog == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil catalog")
	}
	min := cfg.MinScore
	if min == 0 {
		min = defaultTagCurateMin
	}
	var judge harness.Judge
	if cfg.Judge != nil {
		judge = cfg.Judge
	} else if cfg.Client != nil && strings.TrimSpace(cfg.JudgeModel) != "" {
		judge = &polypusJudge{
			jev:   jev.NewClient(cfg.Client.BaseURL, cfg.Client.HTTPClient, ""),
			model: strings.TrimSpace(cfg.JudgeModel),
		}
	}
	inner, err := tags.CreateCurator(tags.CuratorConfig{
		Ledger:       shapeLedger{store: cfg.Store},
		Catalog:      cfg.Catalog,
		Objective:    mailTagObjective(),
		Judge:        judge,
		MinScore:     min,
		StoreTimeout: tagCurateStoreTimeout,
		JudgeTimeout: tagCurateJevTimeout,
	})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeInvalid, op, "create curator")
	}
	return &TagCurator{inner: inner}, nil
}

// Catalog returns the catalog last loaded by the curator.
func (c *TagCurator) Catalog() *tags.Catalog {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.Catalog()
}

// Run applies deterministic filter rewrite, optional Jev maps, and register regeneration.
func (c *TagCurator) Run(parent context.Context) (CurateResult, error) {
	const op = "mailreport.TagCurator.Run"
	if c == nil || c.inner == nil {
		return CurateResult{}, sirerr.New(sirerr.CodeInvalid, op, "nil curator")
	}
	if parent == nil {
		parent = context.Background()
	}
	if err := parent.Err(); err != nil {
		return CurateResult{}, err
	}
	if _, ok := parent.Deadline(); !ok {
		var cancel context.CancelFunc
		parent, cancel = context.WithTimeout(parent, tagCurateJevTimeout+tagCurateStoreTimeout*2)
		defer cancel()
	}
	res, err := c.inner.Run(parent)
	if err != nil {
		return res, sirerr.Wrap(err, sirerr.CodeFailed, op, "curate")
	}
	return res, nil
}
