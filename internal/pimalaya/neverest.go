package pimalaya

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Neverest wraps neverest check/sync.
type Neverest struct {
	Runner *Runner
}

// CreateNeverest resolves the binary and config from host config.
func CreateNeverest(cfg config.Config) (*Neverest, error) {
	const op = "pimalaya.CreateNeverest"
	bin, err := ResolveBin(cfg.Pimalaya.NeverestBin, "neverest")
	if err != nil {
		return nil, err
	}
	runner, err := Create(bin)
	if err != nil {
		return nil, err
	}
	runner.ConfigPath = strings.TrimSpace(cfg.Pimalaya.NeverestConfig)
	return &Neverest{Runner: runner}, nil
}

// Check runs neverest check.
func (n *Neverest) Check(ctx context.Context) (*Result, error) {
	if n == nil || n.Runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, "pimalaya.Neverest.Check", "nil neverest")
	}
	return n.Runner.RunJSON(ctx, "check")
}

// Sync runs neverest sync for an optional account id.
func (n *Neverest) Sync(ctx context.Context, account string) (*Result, error) {
	const op = "pimalaya.Neverest.Sync"
	if n == nil || n.Runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil neverest")
	}
	prev := n.Runner.Account
	if strings.TrimSpace(account) != "" {
		n.Runner.Account = strings.TrimSpace(account)
	}
	defer func() { n.Runner.Account = prev }()
	return n.Runner.RunJSON(ctx, "sync")
}

// Init runs neverest init for the configured account.
func (n *Neverest) Init(ctx context.Context, account string) (*Result, error) {
	const op = "pimalaya.Neverest.Init"
	if n == nil || n.Runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil neverest")
	}
	prev := n.Runner.Account
	if strings.TrimSpace(account) != "" {
		n.Runner.Account = strings.TrimSpace(account)
	}
	defer func() { n.Runner.Account = prev }()
	res, err := n.Runner.RunJSON(ctx, "init")
	if err == nil {
		return res, nil
	}
	if replicaAlreadyInitialized(err, res) {
		return res, nil
	}
	return res, err
}

func replicaAlreadyInitialized(err error, res *Result) bool {
	if err == nil {
		return true
	}
	var parts []string
	if res != nil {
		parts = append(parts, res.Stderr, res.Stdout, string(res.Raw))
	}
	parts = append(parts, err.Error())
	joined := strings.ToLower(strings.Join(parts, " "))
	for _, needle := range []string{"already exists", "already initialized", "refuses to run if it already exists"} {
		if strings.Contains(joined, needle) {
			return true
		}
	}
	return false
}
