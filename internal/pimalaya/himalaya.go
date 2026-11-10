package pimalaya

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Himalaya wraps himalaya message operations.
type Himalaya struct {
	Runner *Runner
}

// CreateHimalaya resolves the binary and config from host config.
func CreateHimalaya(cfg config.Config) (*Himalaya, error) {
	const op = "pimalaya.CreateHimalaya"
	bin, err := ResolveBin(cfg.Pimalaya.HimalayaBin, "himalaya")
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "resolve himalaya bin")
	}
	runner, err := Create(bin)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create himalaya runner")
	}
	runner.ConfigPath = strings.TrimSpace(cfg.Pimalaya.HimalayaConfig)
	runner.Account = strings.TrimSpace(cfg.Pimalaya.DefaultAccount)
	runner.ExtraEnv = cfg.OAuthEnv()
	return &Himalaya{Runner: runner}, nil
}

// MessageDelete moves one message to trash when not already in trash.
func (h *Himalaya) MessageDelete(ctx context.Context, folder, envelopeID string) (*Result, error) {
	const op = "pimalaya.Himalaya.MessageDelete"
	if h == nil || h.Runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil himalaya")
	}
	folder = strings.TrimSpace(folder)
	env := strings.TrimSpace(envelopeID)
	if folder == "" || env == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "folder and envelope id required")
	}
	return h.Runner.RunJSON(ctx, "message", "delete", "-m", folder, env)
}
