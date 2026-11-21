package mailreport

import (
	"context"
	"strconv"
	"strings"

	"github.com/behaviorengineering/strop/pkg/openaibatch"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const tldrSystemPrompt = "Summarize these inbox messages in one sentence. Report only. Do not advise deleting, moving, or marking mail."

const maxTLDRRunes = 280

// Cluster fills clusters and TLDRs on the artifact.
func (r *PostPasses) Cluster(ctx context.Context, art Artifact) (Artifact, error) {
	const op = "mailreport.Runner.Cluster"
	if r == nil {
		return art, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if ctx == nil {
		return art, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return art, sirerr.New(sirerr.CodeInvalid, op, "missing context deadline")
	}
	if r.store == nil {
		return art, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	ctx = WithPaceGate(ctx, CreatePaceGate(paceMaxRPS))
	art.TldrModel = r.tldrModel
	minCosine := r.clusterMinCosine
	if minCosine <= 0 {
		minCosine = 0.80
	}
	var similar SimilarFunc
	if r.embedModel != "" {
		similar = func(c context.Context, hash, model string, k int) ([]hoststore.Neighbor, error) {
			return r.store.Similar(c, hash, model, k)
		}
	}
	rows, clusters, err := AssignClusters(ctx, art.Messages, AssignOpts{
		MinCosine:  minCosine,
		EmbedModel: r.embedModel,
		Similar:    similar,
	})
	if err != nil {
		return art, sirerr.Wrap(err, sirerr.CodeFailed, op, "assign clusters")
	}
	art.Messages = rows
	rowByHash := map[string]MessageRow{}
	for _, row := range art.Messages {
		rowByHash[row.ObjectHash] = row
	}
	finish := func(retErr error) (Artifact, error) {
		for i := range clusters {
			r.applyClusterAttention(&clusters[i], rowByHash)
		}
		SortClustersByAttention(clusters)
		art.Clusters = clusters
		return art, retErr
	}
	quarantine := quarantineSet(r.quarantineTerms)
	if err := r.seedClusterTLDRs(ctx, clusters, rowByHash, quarantine); err != nil {
		return finish(err)
	}
	if err := r.fillClusterTLDRs(ctx, &clusters, rowByHash, quarantine); err != nil {
		return finish(err)
	}
	return finish(nil)
}

func (r *PostPasses) seedClusterTLDRs(ctx context.Context, clusters []ClusterRow, rowByHash map[string]MessageRow, quarantine map[string]struct{}) error {
	const op = "mailreport.Runner.Cluster"
	for i := range clusters {
		c := &clusters[i]
		c.SuggestedAction = suggestedActionFor(c.TermID, quarantine)
		if c.Size == 1 {
			c.TLDR = singleMessageTLDR(rowByHash, c.MemberHashes)
			if err := r.persistCluster(ctx, *c, rowByHash); err != nil {
				return err
			}
			continue
		}
		got, gerr := r.store.GetClusterByFingerprint(ctx, c.Fingerprint)
		if gerr == nil && strings.TrimSpace(got.TLDR) != "" {
			c.TLDR = got.TLDR
			c.TLDRError = ""
			if err := r.persistCluster(ctx, *c, rowByHash); err != nil {
				return err
			}
			continue
		}
		if gerr != nil {
			code, ok := sirerr.AsCode(gerr)
			if !ok || code != sirerr.CodeNotFound {
				return sirerr.Wrap(gerr, sirerr.CodeFailed, op, "get cluster cache")
			}
		}
	}
	return nil
}

func (r *PostPasses) fillClusterTLDRs(ctx context.Context, clusters *[]ClusterRow, rowByHash map[string]MessageRow, quarantine map[string]struct{}) error {
	const op = "mailreport.Runner.fillClusterTLDRs"
	chatLines, chatIDs, err := r.collectTLDRChatLines(ctx, clusters, rowByHash)
	if err != nil {
		return err
	}
	if len(chatLines) == 0 || r.tldrBatch == nil {
		return nil
	}
	results, batchErr := r.tldrBatch.RunChatLines(ctx, chatLines)
	if batchErr != nil {
		return sirerr.Wrap(batchErr, sirerr.CodeFailed, op, "tldr chat batch")
	}
	return r.applyTLDRBatchResults(ctx, clusters, rowByHash, quarantine, chatIDs, results)
}

func (r *PostPasses) collectTLDRChatLines(ctx context.Context, clusters *[]ClusterRow, rowByHash map[string]MessageRow) ([]openaibatch.ChatLine, []string, error) {
	var chatLines []openaibatch.ChatLine
	var chatIDs []string
	for i := range *clusters {
		c := &(*clusters)[i]
		if strings.TrimSpace(c.TLDR) != "" || c.Size == 1 {
			continue
		}
		if r.tldrBatch == nil {
			c.TLDRError = "tldr runner unavailable"
			if err := r.persistCluster(ctx, *c, rowByHash); err != nil {
				return nil, nil, err
			}
			continue
		}
		chatIDs = append(chatIDs, c.ID)
		chatLines = append(chatLines, openaibatch.ChatLine{
			CustomID: c.ID,
			Model:    r.tldrModel,
			Messages: []map[string]string{
				{chatRoleKey: polypus.RoleSystem, chatContentKey: tldrSystemPrompt},
				{chatRoleKey: polypus.RoleUser, chatContentKey: clusterTLDRUserBody(c, rowByHash)},
			},
		})
	}
	return chatLines, chatIDs, nil
}

func (r *PostPasses) applyTLDRBatchResults(ctx context.Context, clusters *[]ClusterRow, rowByHash map[string]MessageRow, quarantine map[string]struct{}, chatIDs []string, results map[string]openaibatch.LineResult) error {
	for _, cid := range chatIDs {
		c := findClusterByID(*clusters, cid)
		if c == nil {
			continue
		}
		applyOneTLDRResult(c, results[cid])
		c.SuggestedAction = suggestedActionFor(c.TermID, quarantine)
		if err := r.persistCluster(ctx, *c, rowByHash); err != nil {
			return err
		}
	}
	return nil
}

func applyOneTLDRResult(c *ClusterRow, res openaibatch.LineResult) {
	if strings.TrimSpace(res.ErrMessage) != "" {
		c.TLDRError = res.ErrMessage
		c.TLDR = ""
		return
	}
	parsed := parseTLDRLine(res.Content)
	if parsed == "" {
		c.TLDRError = "empty tldr"
		c.TLDR = ""
		return
	}
	c.TLDR = parsed
	c.TLDRError = ""
}

func findClusterByID(clusters []ClusterRow, id string) *ClusterRow {
	for i := range clusters {
		if clusters[i].ID == id {
			return &clusters[i]
		}
	}
	return nil
}

func suggestedActionFor(termID string, quarantine map[string]struct{}) string {
	t := strings.TrimSpace(strings.ToLower(termID))
	if t != "" {
		if _, ok := quarantine[t]; ok {
			return SuggestedQuarantineCandidate
		}
	}
	return SuggestedReview
}

func singleMessageTLDR(rows map[string]MessageRow, hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}
	row := rows[hashes[0]]
	if strings.TrimSpace(row.About) != "" {
		return strings.TrimSpace(row.About)
	}
	return strings.TrimSpace(row.Subject)
}

func clusterTLDRUserBody(c *ClusterRow, rows map[string]MessageRow) string {
	const maxLines = 20
	var b strings.Builder
	n := 0
	extra := 0
	for _, hash := range c.MemberHashes {
		row := rows[hash]
		if n >= maxLines {
			extra++
			continue
		}
		n++
		b.WriteString(truncateRunes(strings.TrimSpace(row.Subject), 120))
		b.WriteString(" | ")
		b.WriteString(truncateRunes(strings.TrimSpace(row.Sender), 80))
		b.WriteString(" | ")
		b.WriteString(truncateRunes(strings.TrimSpace(row.About), 240))
		b.WriteString(" | ")
		b.WriteString(truncateRunes(strings.TrimSpace(row.Shape), 80))
		b.WriteByte('\n')
	}
	if extra > 0 {
		b.WriteString("... and ")
		b.WriteString(strconv.Itoa(extra))
		b.WriteString(" more")
	}
	return b.String()
}

func parseTLDRLine(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		ln = strings.Trim(ln, "\"'")
		if ln != "" {
			return truncateRunes(ln, maxTLDRRunes)
		}
	}
	return ""
}

func (r *PostPasses) applyClusterAttention(c *ClusterRow, rowByHash map[string]MessageRow) {
	if c == nil {
		return
	}
	members := make([]MessageRow, 0, len(c.MemberHashes))
	for _, h := range c.MemberHashes {
		members = append(members, rowByHash[h])
	}
	tm := r.attentionTrueMin
	if tm == 0 {
		tm = defaultAttentionTrueMin
	}
	c.Attention = RollupAttention(members, tm)
}

func (r *PostPasses) persistCluster(ctx context.Context, c ClusterRow, rowByHash map[string]MessageRow) error {
	const op = "mailreport.Runner.persistCluster"
	r.applyClusterAttention(&c, rowByHash)
	rec := hoststore.ClusterRecord{
		ClusterID:        c.ID,
		Fingerprint:      c.Fingerprint,
		TermID:           c.TermID,
		SenderDomain:     c.SenderDomain,
		Size:             c.Size,
		TLDR:             c.TLDR,
		SuggestedAction:  c.SuggestedAction,
		TLDRModel:        r.tldrModel,
		Urgency:          0,
		NeedsReply:       false,
		InvolvesDecision: false,
		AttentionScored:  false,
		MemberHashes:     c.MemberHashes,
	}
	if c.Attention != nil {
		rec.Urgency = c.Attention.Urgency
		rec.NeedsReply = c.Attention.NeedsReply
		rec.InvolvesDecision = c.Attention.InvolvesDecision
		rec.AttentionScored = true
	}
	if err := r.store.PutCluster(ctx, rec); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "put cluster")
	}
	return nil
}
