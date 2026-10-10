package mailreport

import (
	"context"
	"strings"

	"github.com/behaviorengineering/strop/pkg/dspy/subllm"
	"github.com/behaviorengineering/strop/pkg/openaibatch"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	dspyrlm "github.com/XiaoConstantine/dspy-go/pkg/modules/rlm"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type noopSubLLM struct{}

// NoopSubLLM satisfies PreferBatch sync fallback (RunChatLines uses HTTP sync path).
func NoopSubLLM() dspyrlm.SubLLMClient { return noopSubLLM{} }

func (noopSubLLM) Query(context.Context, string) (dspyrlm.QueryResponse, error) {
	return dspyrlm.QueryResponse{}, sirerr.New(sirerr.CodeFailed, "mailreport.noopSubLLM.Query", "not implemented")
}

func (noopSubLLM) QueryBatched(ctx context.Context, prompts []string) ([]dspyrlm.QueryResponse, error) {
	out := make([]dspyrlm.QueryResponse, len(prompts))
	for i := range prompts {
		out[i] = dspyrlm.QueryResponse{Response: ""}
	}
	return out, nil
}

// ChatLineRunner runs batched chat lines (PreferBatch.RunChatLines).
type ChatLineRunner interface {
	RunChatLines(ctx context.Context, lines []openaibatch.ChatLine) (map[string]openaibatch.LineResult, error)
}

// NewPreferBatchChatRunner wraps Polypus batch chat for essence prefetch.
func NewPreferBatchChatRunner(sync dspyrlm.SubLLMClient, client openaibatch.Client, model string) (ChatLineRunner, error) {
	const op = "mailreport.NewPreferBatchChatRunner"
	pb := subllm.NewPreferBatch(sync, client, model)
	if pb == nil {
		return nil, sirerr.New(sirerr.CodeFailed, op, "nil prefer batch client")
	}
	runner, ok := pb.(*subllm.PreferBatch)
	if !ok {
		return nil, sirerr.New(sirerr.CodeFailed, op, "prefer batch type mismatch")
	}
	return runner, nil
}

func prefetchEssence(
	ctx context.Context,
	runner ChatLineRunner,
	model string,
	world string,
	items []Item,
	batchSize int,
) (map[string]harness.EssenceOut, map[string]error) {
	out := map[string]harness.EssenceOut{}
	errs := map[string]error{}
	if runner == nil || batchSize <= 0 || len(items) == 0 {
		return out, errs
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return out, errs
	}
	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		chunk := items[start:end]
		lines := make([]openaibatch.ChatLine, 0, len(chunk))
		for _, it := range chunk {
			hash := strings.TrimSpace(it.ObjectHash)
			if hash == "" {
				continue
			}
			messageText := AuthorMessageBody(formatItemText(it))
			prompt := strings.TrimSpace(world) + "\n\nMessage:\n" + messageText
			lines = append(lines, openaibatch.ChatLine{
				CustomID: hash,
				Model:    model,
				Messages: []map[string]string{
					{"role": polypus.RoleSystem, "content": essenceSystemPrompt},
					{"role": polypus.RoleUser, "content": prompt},
				},
			})
		}
		if len(lines) == 0 {
			continue
		}
		results, err := runner.RunChatLines(ctx, lines)
		if err != nil {
			for _, ln := range lines {
				errs[ln.CustomID] = err
			}
			continue
		}
		for _, ln := range lines {
			id := ln.CustomID
			lr, ok := results[id]
			if !ok {
				errs[id] = sirerr.New(sirerr.CodeAI, "mailreport.prefetchEssence", "missing batch result")
				continue
			}
			if strings.TrimSpace(lr.ErrMessage) != "" {
				errs[id] = sirerr.New(sirerr.CodeAI, "mailreport.prefetchEssence", lr.ErrMessage)
				continue
			}
			parsed, perr := parseEssenceResponse(lr.Content)
			if perr == nil {
				out[id] = parsed
				continue
			}
			// Retry one line (sync path inside PreferBatch for len==1).
			retry, rerr := runner.RunChatLines(ctx, []openaibatch.ChatLine{ln})
			if rerr != nil {
				errs[id] = perr
				continue
			}
			lr2, ok := retry[id]
			if !ok || strings.TrimSpace(lr2.ErrMessage) != "" {
				errs[id] = perr
				continue
			}
			parsed2, perr2 := parseEssenceResponse(lr2.Content)
			if perr2 != nil {
				errs[id] = perr2
				continue
			}
			out[id] = parsed2
		}
	}
	return out, errs
}
