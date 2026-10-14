package mailreport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/mail"
	"regexp"
	"sort"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

var senderEmailRE = regexp.MustCompile(`[^@\s]+@[^>\s]+`)

// SimilarFunc loads embedding neighbors for attach grouping.
type SimilarFunc func(ctx context.Context, objectHash, modelID string, k int) ([]hoststore.Neighbor, error)

// AssignOpts configures cluster assignment.
type AssignOpts struct {
	MinCosine  float64
	EmbedModel string
	Similar    SimilarFunc
}

type clusterBucket struct {
	groupKey string
	termID   string
	domain   string
	members  []MessageRow
}

// AssignClusters groups classified rows and assigns cluster_id without chat or store writes.
func AssignClusters(ctx context.Context, rows []MessageRow, opts AssignOpts) ([]MessageRow, []ClusterRow, error) {
	const op = "mailreport.AssignClusters"
	if ctx == nil {
		return nil, nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if opts.Similar != nil {
		if _, ok := ctx.Deadline(); !ok {
			return nil, nil, sirerr.New(sirerr.CodeInvalid, op, "missing context deadline")
		}
	}
	buckets := map[string]*clusterBucket{}
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "assign canceled")
		}
		if strings.TrimSpace(row.ObjectHash) == "" || strings.TrimSpace(row.Error) != "" {
			continue
		}
		gk, term, domain := groupKeyForRow(row)
		b, ok := buckets[gk]
		if !ok {
			b = &clusterBucket{groupKey: gk, termID: term, domain: domain}
			buckets[gk] = b
			order = append(order, gk)
		}
		b.members = append(b.members, row)
	}
	if opts.Similar != nil && strings.TrimSpace(opts.EmbedModel) != "" && opts.MinCosine > 0 {
		if err := attachSimilar(ctx, buckets, opts); err != nil {
			return nil, nil, err
		}
		order = make([]string, 0, len(buckets))
		for gk := range buckets {
			order = append(order, gk)
		}
		sort.Strings(order)
	}
	clusters := make([]ClusterRow, 0, len(buckets))
	outRows := append([]MessageRow{}, rows...)
	hashToCluster := map[string]string{}
	for _, gk := range order {
		b := buckets[gk]
		if b == nil || len(b.members) == 0 {
			continue
		}
		cid := clusterIDFromGroupKey(gk)
		members := b.members
		sort.Slice(members, func(i, j int) bool {
			return members[i].ObjectHash < members[j].ObjectHash
		})
		hashes := make([]string, 0, len(members))
		for _, m := range members {
			hashes = append(hashes, m.ObjectHash)
			hashToCluster[m.ObjectHash] = cid
		}
		fp := clusterFingerprint(gk, hashes)
		clusters = append(clusters, ClusterRow{
			ID:              cid,
			TermID:          b.termID,
			SenderDomain:    b.domain,
			Size:            len(hashes),
			SuggestedAction: SuggestedReview,
			MemberHashes:    hashes,
			Fingerprint:     fp,
		})
	}
	for i := range outRows {
		if cid, ok := hashToCluster[outRows[i].ObjectHash]; ok {
			outRows[i].ClusterID = cid
		}
	}
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Size != clusters[j].Size {
			return clusters[i].Size > clusters[j].Size
		}
		return clusters[i].ID < clusters[j].ID
	})
	return outRows, clusters, nil
}

func groupKeyForRow(row MessageRow) (gk, termID, domain string) {
	term := strings.TrimSpace(row.CanonicalTermID)
	if term == "" {
		term = strings.TrimSpace(row.TermID)
	}
	if term != "" {
		return "term:" + term, term, ""
	}
	domain = senderDomain(row.Sender)
	if domain != "" {
		return "domain:" + domain, "", domain
	}
	hash := strings.TrimSpace(row.ObjectHash)
	return "unlabeled:" + hash, "", ""
}

func senderDomain(sender string) string {
	s := strings.TrimSpace(sender)
	if s == "" {
		return ""
	}
	if addr, err := mail.ParseAddress(s); err == nil && addr != nil {
		return domainFromEmail(addr.Address)
	}
	if i := strings.LastIndex(s, "<"); i >= 0 && strings.HasSuffix(s, ">") {
		inner := strings.TrimSpace(s[i+1 : len(s)-1])
		if d := domainFromEmail(inner); d != "" {
			return d
		}
	}
	if m := senderEmailRE.FindString(s); m != "" {
		return domainFromEmail(m)
	}
	return ""
}

func domainFromEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	d := strings.TrimRight(email[at+1:], ".")
	return d
}

func clusterIDFromGroupKey(groupKey string) string {
	sum := sha256.Sum256([]byte(groupKey))
	return "c-" + hex.EncodeToString(sum[:])[:12]
}

func clusterFingerprint(groupKey string, memberHashes []string) string {
	sorted := append([]string{}, memberHashes...)
	sort.Strings(sorted)
	unique := make([]string, 0, len(sorted))
	var prev string
	for _, h := range sorted {
		h = strings.TrimSpace(h)
		if h == "" || h == prev {
			continue
		}
		unique = append(unique, h)
		prev = h
	}
	body := "v1\nkey=" + groupKey + "\nmembers=" + strings.Join(unique, ",")
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func attachSimilar(ctx context.Context, buckets map[string]*clusterBucket, opts AssignOpts) error {
	const op = "mailreport.attachSimilar"
	inArtifact := map[string]string{}
	for gk, b := range buckets {
		if b == nil {
			continue
		}
		for _, m := range b.members {
			inArtifact[m.ObjectHash] = gk
		}
	}
	for gk, b := range buckets {
		if b == nil || !strings.HasPrefix(gk, "domain:") && !strings.HasPrefix(gk, "unlabeled:") {
			continue
		}
		var kept []MessageRow
		for _, m := range b.members {
			if err := ctx.Err(); err != nil {
				return sirerr.Wrap(err, sirerr.CodeFailed, op, "similar attach canceled")
			}
			moved := false
			neighbors, err := opts.Similar(ctx, m.ObjectHash, opts.EmbedModel, 5)
			if err != nil {
				kept = append(kept, m)
				continue
			}
			for _, n := range neighbors {
				targetGK, ok := inArtifact[n.ObjectHash]
				if !ok || targetGK == gk || !strings.HasPrefix(targetGK, "term:") {
					continue
				}
				if n.Cosine < opts.MinCosine {
					continue
				}
				tb, ok := buckets[targetGK]
				if !ok || tb == nil {
					continue
				}
				tb.members = append(tb.members, m)
				inArtifact[m.ObjectHash] = targetGK
				moved = true
				break
			}
			if !moved {
				kept = append(kept, m)
			}
		}
		b.members = kept
		if len(b.members) == 0 {
			delete(buckets, gk)
		}
	}
	return nil
}
