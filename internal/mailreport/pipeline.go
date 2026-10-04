package mailreport

import (
	"sync"

	"github.com/behaviorengineering/strop/pkg/dspy/registry"
	"github.com/behaviorengineering/strop/pkg/dspy/runner"
	stroplog "github.com/behaviorengineering/strop/pkg/log"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	dspymod "github.com/XiaoConstantine/dspy-go/pkg/modules"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// PipelineConfig wires strop JobRunner around taxonomy Operate.
type PipelineConfig struct {
	Harness *harness.Harness
	Seats   Seats
	Logger  stroplog.Logger
}

// Pipeline holds JobRunner and pending Operate state for classify.
type Pipeline struct {
	Jobs           *runner.JobRunner
	mu             sync.Mutex
	pendingHarness *harness.Harness
	pendingOp      harness.Op
	outcomeRes     harness.Result
	outcomeErr     error
	hasOutcome     bool
}

// CreatePipeline registers mail_classify and returns a Pipeline with JobRunner.
func CreatePipeline(cfg PipelineConfig) (*Pipeline, error) {
	const op = "mailreport.CreatePipeline"
	if cfg.Harness == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil harness")
	}
	if cfg.Seats.Judge == nil || cfg.Seats.Author == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil judge or author")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = discardLogger{}
	}
	pipe := &Pipeline{}
	reg := registry.NewModuleRegistry()
	llm := &operateLLM{pipe: pipe}
	predict := dspymod.NewPredict(classifySignature()).WithTextOutput()
	predict.SetLLM(llm)
	reg.RegisterGenerator(JobMailClassify, predict)
	jobs := runner.NewJobRunner(reg, nil, nil, logger)
	pipe.Jobs = jobs
	return pipe, nil
}

func classifySignature() core.Signature {
	return core.NewSignature(
		[]core.InputField{
			{Name: fieldObjectHash},
			{Name: fieldText},
		},
		[]core.OutputField{
			{Field: core.NewField(fieldTermID)},
			{Field: core.NewField(fieldLabel)},
			{Field: core.NewField(fieldSource)},
			{Field: core.NewField(fieldJudgeScore)},
			{Field: core.NewField(fieldError)},
			{Field: core.NewField(fieldDraftKind)},
			{Field: core.NewField(fieldDraftID)},
			{Field: core.NewField(fieldLeafID)},
			{Field: core.NewField(fieldAlias)},
		},
	)
}

// SetPending stores the next Operate call for operateLLM.Generate.
func (p *Pipeline) SetPending(h *harness.Harness, op harness.Op) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pendingHarness = h
	p.pendingOp = op
	p.hasOutcome = false
	p.outcomeRes = harness.Result{}
	p.outcomeErr = nil
}

func (p *Pipeline) takePending() (*harness.Harness, harness.Op, error) {
	const opName = "mailreport.Pipeline.takePending"
	if p == nil {
		return nil, harness.Op{}, sirerr.New(sirerr.CodeInvalid, opName, "nil pipeline")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pendingHarness == nil {
		return nil, harness.Op{}, sirerr.New(sirerr.CodeInvalid, opName, "no pending operate")
	}
	h := p.pendingHarness
	pending := p.pendingOp
	return h, pending, nil
}

func (p *Pipeline) storeOutcome(res harness.Result, err error) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.outcomeRes = res
	p.outcomeErr = err
	p.hasOutcome = true
	p.pendingHarness = nil
}

// TakeOutcome returns the last Operate result after Generate.
func (p *Pipeline) TakeOutcome() (harness.Result, error) {
	const op = "mailreport.Pipeline.TakeOutcome"
	if p == nil {
		return harness.Result{}, sirerr.New(sirerr.CodeInvalid, op, "nil pipeline")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.hasOutcome {
		return harness.Result{}, sirerr.New(sirerr.CodeFailed, op, "no operate outcome")
	}
	res, err := p.outcomeRes, p.outcomeErr
	p.hasOutcome = false
	return res, err
}
