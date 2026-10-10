package mailreport

import (
	"context"
	"strconv"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/XiaoConstantine/dspy-go/pkg/core"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// operateLLM implements core.LLM by running taxonomy Operate (Polypus seats inside).
type operateLLM struct {
	pipe *Pipeline
}

func (o *operateLLM) Generate(ctx context.Context, _ string, _ ...core.GenerateOption) (*core.LLMResponse, error) {
	if o == nil || o.pipe == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, "mailreport.operateLLM.Generate", "nil pipeline")
	}
	h, op, err := o.pipe.takePending()
	if err != nil {
		return nil, err
	}
	res, opErr := h.Operate(ctx, op)
	o.pipe.storeOutcome(res, opErr)
	if opErr != nil {
		if isUnavailable(opErr) {
			return nil, opErr
		}
		return &core.LLMResponse{Content: formatOperateContent(res, opErr)}, nil
	}
	return &core.LLMResponse{Content: formatOperateContent(res, nil)}, nil
}

func formatOperateContent(res harness.Result, opErr error) string {
	var b strings.Builder
	writeField(&b, fieldTermID, termIDFromResult(res))
	writeField(&b, fieldLabel, labelFromResult(res))
	writeField(&b, fieldSource, sourceFromResult(res))
	writeField(&b, fieldJudgeScore, strconv.FormatFloat(res.JudgeScore, 'f', -1, 64))
	writeField(&b, "strategy", string(res.Strategy))
	writeField(&b, "kind", res.Kind)
	writeField(&b, "about", res.About)
	writeField(&b, "shape", res.Shape)
	writeField(&b, "cosine", strconv.FormatFloat(res.Cosine, 'f', -1, 64))
	writeField(&b, "canonical_term_id", res.CanonicalID)
	writeField(&b, "reinforced", strconv.FormatBool(res.Reinforced))
	if opErr != nil {
		writeField(&b, fieldError, opErr.Error())
	} else {
		writeField(&b, fieldError, "")
	}
	if res.Draft != nil {
		writeField(&b, fieldDraftKind, res.Draft.Kind)
		writeField(&b, fieldDraftID, res.Draft.ID)
		writeField(&b, fieldLeafID, res.Draft.LeafID)
		writeField(&b, fieldAlias, res.Draft.Alias)
		writeField(&b, fieldDraftParent, res.Draft.Parent)
		writeField(&b, fieldDraftLabel, res.Draft.Label)
	}
	return b.String()
}

func writeField(b *strings.Builder, name, value string) {
	b.WriteString(name)
	b.WriteString(":\n")
	b.WriteString(value)
	b.WriteByte('\n')
}

func termIDFromResult(res harness.Result) string {
	if len(res.Assigned) > 0 {
		return res.Assigned[0].TermID
	}
	return ""
}

func labelFromResult(res harness.Result) string {
	if len(res.Assigned) > 0 {
		return res.Assigned[0].Label
	}
	return ""
}

func sourceFromResult(res harness.Result) string {
	if len(res.Assigned) > 0 {
		return res.Assigned[0].Source
	}
	return ""
}

func (o *operateLLM) GenerateWithJSON(ctx context.Context, prompt string, opts ...core.GenerateOption) (map[string]any, error) {
	return nil, sirerr.New(sirerr.CodeInvalid, "mailreport.operateLLM", "GenerateWithJSON not supported")
}

func (o *operateLLM) GenerateWithFunctions(ctx context.Context, prompt string, functions []map[string]any, options ...core.GenerateOption) (map[string]any, error) {
	return nil, sirerr.New(sirerr.CodeInvalid, "mailreport.operateLLM", "GenerateWithFunctions not supported")
}

func (o *operateLLM) CreateEmbedding(ctx context.Context, input string, options ...core.EmbeddingOption) (*core.EmbeddingResult, error) {
	return nil, sirerr.New(sirerr.CodeInvalid, "mailreport.operateLLM", "CreateEmbedding not supported")
}

func (o *operateLLM) CreateEmbeddings(ctx context.Context, inputs []string, options ...core.EmbeddingOption) (*core.BatchEmbeddingResult, error) {
	return nil, sirerr.New(sirerr.CodeInvalid, "mailreport.operateLLM", "CreateEmbeddings not supported")
}

func (o *operateLLM) StreamGenerate(ctx context.Context, prompt string, opts ...core.GenerateOption) (*core.StreamResponse, error) {
	return nil, sirerr.New(sirerr.CodeInvalid, "mailreport.operateLLM", "StreamGenerate not supported")
}

func (o *operateLLM) GenerateWithContent(ctx context.Context, content []core.ContentBlock, options ...core.GenerateOption) (*core.LLMResponse, error) {
	return nil, sirerr.New(sirerr.CodeInvalid, "mailreport.operateLLM", "GenerateWithContent not supported")
}

func (o *operateLLM) StreamGenerateWithContent(ctx context.Context, content []core.ContentBlock, options ...core.GenerateOption) (*core.StreamResponse, error) {
	return nil, sirerr.New(sirerr.CodeInvalid, "mailreport.operateLLM", "StreamGenerateWithContent not supported")
}

func (o *operateLLM) ProviderName() string { return "taxonomy-operate" }

func (o *operateLLM) ModelID() string { return "taxonomy-operate" }

func (o *operateLLM) Capabilities() []core.Capability {
	return []core.Capability{core.CapabilityChat}
}
