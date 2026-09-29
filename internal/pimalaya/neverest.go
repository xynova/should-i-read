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
	if n == nil || n.Runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, "pimalaya.Neverest.Sync", "nil neverest")
	}
	args := []string{"sync"}
	if strings.TrimSpace(account) != "" {
		args = append(args, account)
	}
	return n.Runner.RunJSON(ctx, args...)
}
