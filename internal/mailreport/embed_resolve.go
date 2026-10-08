package mailreport

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const defaultEmbedModel = "cf_local/@cf/baai/bge-m3"

// ResolveEmbedModel picks an embedding model id and verifies it via strop embed probe.
func ResolveEmbedModel(ctx context.Context, fac StropEmbedFactory, baseURL string, catalogIDs []string, embedYAML string) (string, error) {
	const op = "mailreport.ResolveEmbedModel"
	if fac == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil embed factory")
	}
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", sirerr.New(sirerr.CodeInvalid, op, "context must have a deadline")
	}
	embedYAML = strings.TrimSpace(embedYAML)
	if embedYAML != "" {
		if err := probeEmbedModel(ctx, fac, baseURL, embedYAML); err != nil {
			return "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "configured embed model failed probe").With("model", embedYAML)
		}
		return embedYAML, nil
	}
	if containsModel(catalogIDs, defaultEmbedModel) {
		if err := probeEmbedModel(ctx, fac, baseURL, defaultEmbedModel); err == nil {
			return defaultEmbedModel, nil
		}
	}
	for _, id := range catalogIDs {
		if isJevModelID(id) || isNonChatModelID(id) {
			continue
		}
		if !strings.Contains(strings.ToLower(id), "bge") && !strings.Contains(strings.ToLower(id), "embed") {
			continue
		}
		if err := probeEmbedModel(ctx, fac, baseURL, id); err == nil {
			return id, nil
		}
	}
	return "", sirerr.New(sirerr.CodeUnavailable, op, "no embed model resolved; set polypus.embed_model")
}

func probeEmbedModel(ctx context.Context, fac StropEmbedFactory, baseURL, modelID string) error {
	const op = "mailreport.probeEmbedModel"
	emb, err := fac.CreateEmbedder(ctx, baseURL, modelID)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, op, "create embedder")
	}
	_, err = emb.Embed(ctx, []string{"probe"})
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, op, "probe embed")
	}
	return nil
}

func containsModel(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
