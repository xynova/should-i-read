package pimalaya

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// Result is decoded JSON stdout from a Pimalaya CLI when --json is used.
type Result struct {
	OK     bool            `json:"ok"`
	Data   json.RawMessage `json:"data"`
	Error  json.RawMessage `json:"error"`
	Raw    json.RawMessage `json:"-"`
	Stdout string          `json:"-"`
	Stderr string          `json:"-"`
	Exit   int             `json:"-"`
}

// Runner executes allow-listed Pimalaya binaries.
type Runner struct {
	Bin        string
	ConfigPath string
	Account    string
	ExtraEnv   []string
}

// Create builds a Runner after allow-list and path checks.
func Create(bin string) (*Runner, error) {
	const op = "pimalaya.Create"
	bin = strings.TrimSpace(bin)
	if bin == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "binary path is empty")
	}
	if err := AssertAllowed(bin); err != nil {
		return nil, err
	}
	return &Runner{Bin: bin}, nil
}

// ResolveBin finds an executable: explicit path, PATH, then $CARGO_HOME/bin/<name>
// (or ~/.cargo/bin/<name>). Cargo install often lands there while that dir is off PATH.
func ResolveBin(explicit string, name string) (string, error) {
	const op = "pimalaya.ResolveBin"
	if p := strings.TrimSpace(explicit); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if err := AssertAllowed(p); err != nil {
				return "", err
			}
			return p, nil
		}
		return "", sirerr.New(sirerr.CodeNotFound, op, "explicit binary not found").With("path", p)
	}
	if err := AssertAllowed(name); err != nil {
		return "", err
	}
	p, err := exec.LookPath(name)
	if err == nil {
		return p, nil
	}
	if cargo := cargoHomeBin(name); cargo != "" {
		return cargo, nil
	}
	return "", sirerr.New(sirerr.CodeUnavailable, op, "binary not on PATH").With("name", name)
}

func cargoHomeBin(name string) string {
	home := strings.TrimSpace(os.Getenv("CARGO_HOME"))
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(userHome) == "" {
			return ""
		}
		home = filepath.Join(userHome, ".cargo")
	}
	candidate := filepath.Join(home, "bin", name)
	st, err := os.Stat(candidate)
	if err != nil || st.IsDir() {
		return ""
	}
	if err := AssertAllowed(candidate); err != nil {
		return ""
	}
	return candidate
}

// RunJSON executes with --json and optional config flag.
func (r *Runner) RunJSON(ctx context.Context, args ...string) (*Result, error) {
	const op = "pimalaya.RunJSON"
	if r == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}

	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/pimalaya")
	ctx, span := tracer.Start(ctx, "pimalaya.RunJSON",
		trace.WithAttributes(
			attribute.String("pimalaya.binary", r.Bin),
			attribute.StringSlice("pimalaya.args", args),
		),
	)
	defer span.End()

	cmdArgs := make([]string, 0, len(args)+6)
	if cfg := strings.TrimSpace(r.ConfigPath); cfg != "" {
		cmdArgs = append(cmdArgs, "-c", cfg)
	}
	if acct := strings.TrimSpace(r.Account); acct != "" {
		cmdArgs = append(cmdArgs, "-a", acct)
	}
	cmdArgs = append(cmdArgs, "--json")
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, r.Bin, cmdArgs...)
	cmd.Env = mergeEnv(os.Environ(), r.ExtraEnv)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exit := exitCodeFrom(err, cmd)
	res := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Exit:   exit,
	}
	if len(bytes.TrimSpace(stdout.Bytes())) > 0 {
		res.Raw = json.RawMessage(bytes.TrimSpace(stdout.Bytes()))
		_ = json.Unmarshal(res.Raw, res)
	}

	if exit == 2 {
		span.SetStatus(codes.Error, "needs_review")
		return res, sirerr.New(sirerr.CodeNeedsReview, op, "pimalaya CLI needs human review").
			With("exit_code", fmt.Sprintf("%d", exit)).
			With("stderr", truncate(stderr.String(), 400))
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "exec failed")
		return res, sirerr.Wrap(err, sirerr.CodeFailed, op, "pimalaya CLI failed").
			With("exit_code", fmt.Sprintf("%d", exit)).
			With("stderr", truncate(stderr.String(), 400))
	}
	if len(res.Raw) > 0 {
		var probe struct {
			OK *bool `json:"ok"`
		}
		if json.Unmarshal(res.Raw, &probe) == nil && probe.OK != nil && !*probe.OK && exit == 0 {
			span.SetStatus(codes.Error, "ok=false")
			return res, sirerr.New(sirerr.CodeFailed, op, "pimalaya CLI reported ok=false").
				With("exit_code", fmt.Sprintf("%d", exit))
		}
	}
	span.SetStatus(codes.Ok, "")
	return res, nil
}

func exitCodeFrom(err error, cmd *exec.Cmd) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	if cmd != nil && cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	return 1
}

func mergeEnv(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
