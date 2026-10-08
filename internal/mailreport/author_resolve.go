package mailreport

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// AuthorSource records how the classify author chat model id was chosen.
type AuthorSource string

const (
	AuthorSourceConfig  AuthorSource = "config"
	AuthorSourceCatalog AuthorSource = "catalog"
)

// ChatProber smokes POST /v1/chat/completions for a model id.
type ChatProber interface {
	ProbeChat(ctx context.Context, model string) error
}

// ResolveAuthorModel picks an Author chat model id and verifies it via chat smoke.
func ResolveAuthorModel(ctx context.Context, prober ChatProber, catalogIDs []string, authorYAML string) (model string, source AuthorSource, err error) {
	const op = "mailreport.ResolveAuthorModel"
	if prober == nil {
		return "", "", sirerr.New(sirerr.CodeInvalid, op, "nil prober")
	}
	if ctx == nil {
		return "", "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", "", sirerr.New(sirerr.CodeInvalid, op, "context must have a deadline")
	}

	authorYAML = strings.TrimSpace(authorYAML)
	if authorYAML != "" {
		if probeErr := prober.ProbeChat(ctx, authorYAML); probeErr != nil {
			code := sirerr.CodeUnavailable
			if c, ok := sirerr.AsCode(probeErr); ok {
				code = c
			}
			return "", "", sirerr.Wrap(probeErr, code, op, "configured author model failed chat probe").
				With("model", authorYAML)
		}
		return authorYAML, AuthorSourceConfig, nil
	}

	for _, id := range chatAuthorCandidates(catalogIDs) {
		ok, probeErr := probeAuthor(ctx, prober, id)
		if probeErr != nil {
			return "", "", sirerr.Wrap(probeErr, sirerr.CodeUnavailable, op, "author chat probe")
		}
		if ok {
			return id, AuthorSourceCatalog, nil
		}
	}

	return "", "", sirerr.New(sirerr.CodeUnavailable, op, "no classify chat model resolved; set polypus.classify_model (prefer cf_local/@cf/... on routed gateways), run: should-i-read polypus check --classify")
}

func chatAuthorCandidates(catalogIDs []string) []string {
	var local, other []string
	for _, id := range catalogIDs {
		if isJevModelID(id) || isNonChatModelID(id) {
			continue
		}
		if strings.HasPrefix(id, "cf_local/") {
			local = append(local, id)
			continue
		}
		other = append(other, id)
	}
	return append(local, other...)
}

func probeAuthor(ctx context.Context, prober ChatProber, model string) (ok bool, err error) {
	probeErr := prober.ProbeChat(ctx, model)
	if probeErr == nil {
		return true, nil
	}
	if code, has := sirerr.AsCode(probeErr); has && code == sirerr.CodeAI {
		return false, nil
	}
	return false, probeErr
}
