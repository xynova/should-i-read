package pimalaya

import (
	"context"
	"os"
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
	cfgPath := strings.TrimSpace(cfg.Pimalaya.HimalayaConfig)
	if cfgPath == "" {
		return nil, sirerr.New(sirerr.CodeUnavailable, op, "pimalaya.himalaya_config is unset; run should-i-read configure")
	}
	if st, statErr := os.Stat(cfgPath); statErr != nil || st.IsDir() {
		return nil, sirerr.New(sirerr.CodeUnavailable, op, "himalaya config file missing; run should-i-read configure").
			With("path", cfgPath)
	}
	runner, err := Create(bin)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "create himalaya runner")
	}
	runner.ConfigPath = cfgPath
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

// FlagSeen is the Himalaya IANA flag for a read message.
const FlagSeen = "seen"

// FlagAdd adds one allow-listed flag. Product Acked only writes seen.
func (h *Himalaya) FlagAdd(ctx context.Context, folder, envelopeID, flag string) (*Result, error) {
	const op = "pimalaya.Himalaya.FlagAdd"
	if h == nil || h.Runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil himalaya")
	}
	folder = strings.TrimSpace(folder)
	env := strings.TrimSpace(envelopeID)
	flag = strings.TrimSpace(flag)
	if folder == "" || env == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "folder and envelope id required")
	}
	if flag != FlagSeen {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "flag not allowed").With("flag", flag)
	}
	return h.Runner.RunJSON(ctx, "flag", "add", "-m", folder, "--flag", flag, env)
}
