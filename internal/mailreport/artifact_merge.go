package mailreport

import "strings"

// FilterArtifactByHashes returns a copy of art containing only rows whose object_hash is in hashes.
func FilterArtifactByHashes(art Artifact, hashes map[string]struct{}) Artifact {
	out := art
	out.Messages = nil
	for _, m := range art.Messages {
		if _, ok := hashes[strings.TrimSpace(m.ObjectHash)]; ok {
			out.Messages = append(out.Messages, m)
		}
	}
	return out
}

// MergeMessageRows copies fields from src rows into dst rows with the same object_hash.
func MergeMessageRows(dst *Artifact, src Artifact) {
	if dst == nil {
		return
	}
	byHash := map[string]MessageRow{}
	for _, m := range src.Messages {
		byHash[strings.TrimSpace(m.ObjectHash)] = m
	}
	for i := range dst.Messages {
		h := strings.TrimSpace(dst.Messages[i].ObjectHash)
		if u, ok := byHash[h]; ok {
			dst.Messages[i] = u
		}
	}
}

// MergeClassifyArtifacts appends slice classify stats and messages into dst.
func MergeClassifyArtifacts(dst *Artifact, src Artifact) {
	if dst == nil {
		return
	}
	copyClassifyEnvelope(dst, src)
	dst.Messages = append(dst.Messages, src.Messages...)
	dst.Classified += src.Classified
	dst.NewlyClassified += src.NewlyClassified
	dst.Omitted += src.Omitted
	dst.Unresolved += src.Unresolved
	dst.AliasesAdded += src.AliasesAdded
	dst.LeavesAdded += src.LeavesAdded
	if dst.IncompleteError == "" && src.IncompleteError != "" {
		dst.IncompleteError = src.IncompleteError
	}
}

func copyClassifyEnvelope(dst *Artifact, src Artifact) {
	if dst.Mode == "" {
		dst.Mode = src.Mode
	}
	if dst.PolypusBaseURL == "" {
		dst.PolypusBaseURL = src.PolypusBaseURL
	}
	if dst.JudgeModel == "" {
		dst.JudgeModel = src.JudgeModel
	}
	if dst.AuthorModel == "" {
		dst.AuthorModel = src.AuthorModel
	}
	if dst.EmbedModel == "" {
		dst.EmbedModel = src.EmbedModel
	}
	if dst.AttachMinCosine == 0 {
		dst.AttachMinCosine = src.AttachMinCosine
	}
	if dst.WalkReinforceMin == 0 {
		dst.WalkReinforceMin = src.WalkReinforceMin
	}
	if dst.CatalogID == "" {
		dst.CatalogID = src.CatalogID
	}
	if dst.SendersCatalogID == "" {
		dst.SendersCatalogID = src.SendersCatalogID
	}
	if dst.GeneratedAt == "" {
		dst.GeneratedAt = src.GeneratedAt
	}
	if dst.Source == "" {
		dst.Source = src.Source
	}
	if dst.TldrModel == "" {
		dst.TldrModel = src.TldrModel
	}
	if dst.EssenceEscalateModel == "" {
		dst.EssenceEscalateModel = src.EssenceEscalateModel
	}
	if dst.AttentionTrueMin == 0 {
		dst.AttentionTrueMin = src.AttentionTrueMin
	}
	if dst.EssenceDumpDir == "" {
		dst.EssenceDumpDir = src.EssenceDumpDir
	}
}
