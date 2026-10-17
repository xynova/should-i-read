package mailreport

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// MergeClusters assigns clusters globally, reuses TLDR from slice clusters or store, then persists.
func (r *Runner) MergeClusters(ctx context.Context, all []MessageRow, sliceClusters [][]ClusterRow) ([]MessageRow, []ClusterRow, error) {
	const op = "mailreport.Runner.MergeClusters"
	if r == nil {
		return nil, nil, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if ctx == nil {
		return nil, nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, nil, sirerr.New(sirerr.CodeInvalid, op, "missing context deadline")
	}
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
	rows, clusters, err := AssignClusters(ctx, all, AssignOpts{
		MinCosine:  minCosine,
		EmbedModel: r.embedModel,
		Similar:    similar,
	})
	if err != nil {
		return nil, nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "assign clusters")
	}
	tldrByFP := map[string]string{}
	tldrByID := map[string]string{}
	for _, slice := range sliceClusters {
		for _, c := range slice {
			if strings.TrimSpace(c.TLDR) != "" {
				tldrByID[c.ID] = c.TLDR
			}
			fp := strings.TrimSpace(c.Fingerprint)
			if fp == "" || strings.TrimSpace(c.TLDR) == "" {
				continue
			}
			tldrByFP[fp] = c.TLDR
		}
	}
	rowByHash := map[string]MessageRow{}
	for _, row := range rows {
		rowByHash[row.ObjectHash] = row
	}
	quarantine := map[string]struct{}{}
	for _, t := range r.quarantineTerms {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			quarantine[t] = struct{}{}
		}
	}
	for i := range clusters {
		c := &clusters[i]
		c.SuggestedAction = suggestedActionFor(c.TermID, quarantine)
		if c.Size == 1 {
			c.TLDR = singleMessageTLDR(rowByHash, c.MemberHashes)
			if err := r.persistCluster(ctx, *c, rowByHash); err != nil {
				return rows, clusters, err
			}
			continue
		}
		if tldr := tldrByFP[c.Fingerprint]; tldr != "" {
			c.TLDR = tldr
			if err := r.persistCluster(ctx, *c, rowByHash); err != nil {
				return rows, clusters, err
			}
			continue
		}
		if tldr := tldrByID[c.ID]; tldr != "" {
			c.TLDR = tldr
			if err := r.persistCluster(ctx, *c, rowByHash); err != nil {
				return rows, clusters, err
			}
			continue
		}
		got, gerr := r.store.GetClusterByFingerprint(ctx, c.Fingerprint)
		if gerr == nil && strings.TrimSpace(got.TLDR) != "" {
			c.TLDR = got.TLDR
			if err := r.persistCluster(ctx, *c, rowByHash); err != nil {
				return rows, clusters, err
			}
			continue
		}
	}
	if err := r.fillClusterTLDRs(ctx, &clusters, rowByHash, quarantine); err != nil {
		return rows, clusters, err
	}
	for i := range clusters {
		r.applyClusterAttention(&clusters[i], rowByHash)
	}
	SortClustersByAttention(clusters)
	return rows, clusters, nil
}
