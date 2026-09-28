package setup

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// execRunner runs host commands for advanced DIY.
type execRunner struct{}

func (execRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	const op = "setup.runner.Run"
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "context is nil")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", sirerr.New(sirerr.CodeInvalid, op, "caller deadline required before process work")
	}
	if err := ctx.Err(); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		// Never include full stderr if it might contain secrets; keep short.
		msg := "command failed"
		if se := strings.TrimSpace(stderr.String()); se != "" && len(se) < 200 {
			msg = se
		}
		return out, sirerr.Wrap(err, sirerr.CodeFailed, op, msg).With("cmd", name)
	}
	return out, nil
}

// runGCloud attempts optional gcloud DIY. On any gap it fail-closes to guided.
func (w *Wizard) runGCloud(ctx context.Context, res *Result) error {
	const op = "setup.runGCloud"
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "context is nil")
	}
	if _, ok := ctx.Deadline(); !ok {
		return sirerr.New(sirerr.CodeInvalid, op, "caller deadline required before gcloud work")
	}
	if w.runner == nil {
		res.Notes = append(res.Notes, "gcloud runner unset; falling back to guided DIY.")
		return w.runGuided(ctx, res)
	}
	if _, err := w.runner.LookPath("gcloud"); err != nil {
		res.Notes = append(res.Notes, "gcloud not on PATH; falling back to guided DIY.")
		return w.runGuided(ctx, res)
	}

	// Probe auth; any failure → guided (do not invent projects for the operator).
	if _, err := w.runner.Run(ctx, "gcloud", "auth", "list", "--format=value(account)"); err != nil {
		res.Notes = append(res.Notes, "gcloud auth not ready; falling back to guided DIY.")
		return w.runGuided(ctx, res)
	}

	res.Notes = append(res.Notes,
		"gcloud is available, but automatic OAuth client create is not supported for Desktop clients via CLI alone.",
		"Falling back to guided DIY (open Console, create Desktop OAuth client, then paste).",
	)
	return w.runGuided(ctx, res)
}
