package mailreport

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// MergeClusters assigns clusters globally, reuses TLDR from slice clusters or store, then persists.
func (r *PostPasses) MergeClusters(ctx context.Context, all []MessageRow, sliceClusters [][]ClusterRow) ([]MessageRow, []ClusterRow, error) {
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
	tldrByFP, tldrByID := indexSliceTLDRs(sliceClusters)
	rowByHash := map[string]MessageRow{}
	for _, row := range rows {
		rowByHash[row.ObjectHash] = row
	}
	quarantine := quarantineSet(r.quarantineTerms)
	if err := r.reuseMergedTLDRs(ctx, clusters, rowByHash, quarantine, tldrByFP, tldrByID); err != nil {
		return rows, clusters, err
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

func indexSliceTLDRs(sliceClusters [][]ClusterRow) (byFP, byID map[string]string) {
	byFP = map[string]string{}
	byID = map[string]string{}
	for _, slice := range sliceClusters {
		for _, c := range slice {
			if strings.TrimSpace(c.TLDR) != "" {
				byID[c.ID] = c.TLDR
			}
			fp := strings.TrimSpace(c.Fingerprint)
			if fp == "" || strings.TrimSpace(c.TLDR) == "" {
				continue
			}
			byFP[fp] = c.TLDR
		}
	}
	return byFP, byID
}

func quarantineSet(terms []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range terms {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			out[t] = struct{}{}
		}
	}
	return out
}

func (r *PostPasses) reuseMergedTLDRs(ctx context.Context, clusters []ClusterRow, rowByHash map[string]MessageRow, quarantine map[string]struct{}, tldrByFP, tldrByID map[string]string) error {
	for i := range clusters {
		if err := r.reuseOneMergedTLDR(ctx, &clusters[i], rowByHash, quarantine, tldrByFP, tldrByID); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostPasses) reuseOneMergedTLDR(ctx context.Context, c *ClusterRow, rowByHash map[string]MessageRow, quarantine map[string]struct{}, tldrByFP, tldrByID map[string]string) error {
	c.SuggestedAction = suggestedActionFor(c.TermID, quarantine)
	if c.Size == 1 {
		c.TLDR = singleMessageTLDR(rowByHash, c.MemberHashes)
		return r.persistCluster(ctx, *c, rowByHash)
	}
	if tldr := tldrByFP[c.Fingerprint]; tldr != "" {
		c.TLDR = tldr
		return r.persistCluster(ctx, *c, rowByHash)
	}
	if tldr := tldrByID[c.ID]; tldr != "" {
		c.TLDR = tldr
		return r.persistCluster(ctx, *c, rowByHash)
	}
	got, gerr := r.store.GetClusterByFingerprint(ctx, c.Fingerprint)
	if gerr != nil || strings.TrimSpace(got.TLDR) == "" {
		return nil
	}
	c.TLDR = got.TLDR
	return r.persistCluster(ctx, *c, rowByHash)
}
