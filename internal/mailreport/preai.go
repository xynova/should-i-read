package mailreport

import (
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"

	"github.com/xynova/should-i-read/internal/pimdir"
)

// Pre-AI assignment sources.
const (
	SourceHeuristic     = "heuristic"
	SourceSenderCatalog = "sender_catalog"
)

var allowedSenderMapsTo = map[string]struct{}{
	"notification": {},
	"newsletter":   {},
	"promo":        {},
}

var learnInboxLeaves = map[string]struct{}{
	"notification": {},
	"newsletter":   {},
	"promo":        {},
}

// PreAssign is a non-Operate inbox-mail assignment.
type PreAssign struct {
	TermID string
	Label  string
	Path   []string
	Source string
}

// HeuristicAssign applies host RFC822 rules when confident.
func HeuristicAssign(h pimdir.MailHeaders, senderLine string, cat *catalog.Catalog) (PreAssign, bool) {
	if cat == nil {
		return PreAssign{}, false
	}
	from := strings.ToLower(firstNonEmpty(h.From, senderLine))
	if strings.Contains(from, "notifications@github.com") || strings.Contains(from, "noreply@github.com") {
		return leafAssign(cat, "notification", SourceHeuristic)
	}
	auto := strings.ToLower(strings.TrimSpace(h.AutoSubmitted))
	prec := strings.ToLower(strings.TrimSpace(h.Precedence))
	bulk := auto != "" && auto != "no"
	bulk = bulk || prec == "bulk" || prec == "list" || prec == "junk"
	listID := strings.TrimSpace(h.ListID)
	if bulk {
		if listID != "" {
			return leafAssign(cat, "newsletter", SourceHeuristic)
		}
		return leafAssign(cat, "notification", SourceHeuristic)
	}
	if listID != "" {
		return leafAssign(cat, "newsletter", SourceHeuristic)
	}
	return PreAssign{}, false
}

func leafAssign(cat *catalog.Catalog, leafID, source string) (PreAssign, bool) {
	rt, ok := cat.PreferLeaf(leafID)
	if !ok || !cat.IsLeaf(rt.ID) {
		return PreAssign{}, false
	}
	return PreAssign{
		TermID: rt.ID,
		Label:  rt.Label,
		Path:   rt.Path,
		Source: source,
	}, true
}

// SenderCatalogAssign fits the senders vocabulary and maps to an inbox-mail leaf.
func SenderCatalogAssign(senders *catalog.Catalog, inbox *catalog.Catalog, fields map[string]string) (PreAssign, bool) {
	if senders == nil || inbox == nil || len(fields) == 0 {
		return PreAssign{}, false
	}
	rt, ok, err := senders.MatchFields(fields)
	if err != nil || !ok {
		return PreAssign{}, false
	}
	mapTo := strings.TrimSpace(rt.MapsTo)
	if mapTo == "" {
		return PreAssign{}, false
	}
	if _, ok := allowedSenderMapsTo[mapTo]; !ok {
		return PreAssign{}, false
	}
	leaf, ok := leafAssign(inbox, mapTo, SourceSenderCatalog)
	if !ok {
		return PreAssign{}, false
	}
	return leaf, true
}

// MatchSenders returns the senders term that fits fields (catalog MatchFields).
func MatchSenders(senders *catalog.Catalog, fields map[string]string) (catalog.ResolvedTerm, bool) {
	if senders == nil || len(fields) == 0 {
		return catalog.ResolvedTerm{}, false
	}
	rt, ok, err := senders.MatchFields(fields)
	if err != nil || !ok {
		return catalog.ResolvedTerm{}, false
	}
	return rt, true
}

// stampSender sets sender_* on row from MatchSenders when a senders term fits.
func stampSender(row *MessageRow, senders *catalog.Catalog, fields map[string]string) {
	if row == nil {
		return
	}
	rt, ok := MatchSenders(senders, fields)
	if !ok {
		return
	}
	row.SenderTermID = rt.ID
	row.SenderLabel = rt.Label
	row.SenderMapsTo = strings.TrimSpace(rt.MapsTo)
}

func progressEntryFromRow(row MessageRow) ProgressEntry {
	return ProgressEntry{
		TermID:          row.TermID,
		Path:            row.Path,
		Label:           row.Label,
		Source:          row.Source,
		Strategy:        row.Strategy,
		Kind:            row.Kind,
		About:           row.About,
		Shape:           row.Shape,
		Cosine:          row.Cosine,
		CanonicalTermID: row.CanonicalTermID,
		Reinforced:      row.Reinforced,
		JudgeScore:      row.JudgeScore,
		DraftKind:       draftKind(row.Draft),
		CatalogApplied:  row.CatalogApplied,
		Error:           row.Error,
		SenderTermID:    row.SenderTermID,
		SenderLabel:     row.SenderLabel,
		SenderMapsTo:    row.SenderMapsTo,
		UpdatedAt:       progressNow(),
	}
}

// LearnFieldsForSenders returns only from/list_id keys allowed for LearnExact.
func LearnFieldsForSenders(fields map[string]string) map[string]string {
	out := map[string]string{}
	if v := strings.TrimSpace(fields["from"]); v != "" {
		out["from"] = v
	}
	if v := strings.TrimSpace(fields["list_id"]); v != "" {
		out["list_id"] = v
	}
	return out
}

// ShouldLearnSenders reports whether an inbox-mail leaf may update the senders catalog.
func ShouldLearnSenders(inboxLeaf string) bool {
	_, ok := learnInboxLeaves[strings.TrimSpace(inboxLeaf)]
	return ok
}

// MintSenderTermID builds a stable senders vocabulary term id from a from address.
func MintSenderTermID(from string) string {
	from = strings.ToLower(strings.TrimSpace(from))
	if i := strings.LastIndex(from, "<"); i >= 0 {
		from = strings.TrimSuffix(strings.TrimPrefix(from[i:], "<"), ">")
	}
	from = strings.TrimSpace(from)
	if at := strings.LastIndex(from, "@"); at > 0 {
		from = from[:at] + "-" + from[at+1:]
	}
	var b strings.Builder
	b.Grow(len(from))
	prevDash := false
	for _, r := range from {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = "unknown-sender"
	}
	return "sender-" + s
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
