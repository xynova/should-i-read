package emailops

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

	"github.com/xynova/should-i-read/internal/sirerr"
)

// Envelope is the stable emailops-cli --json stdout shape.
type Envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *CLIError       `json:"error"`
}

// CLIError is the failure object inside Envelope.
type CLIError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Params  json.RawMessage `json:"params"`
}

// Client runs emailops-cli against a fixed data directory.
type Client struct {
	Bin     string
	DataDir string
	Quiet   bool
	// ExtraEnv is KEY=VALUE pairs merged into the child process environment.
	ExtraEnv []string
}

// Create returns a Client. bin and dataDir must be non-empty.
func Create(bin, dataDir string) (*Client, error) {
	const op = "emailops.Create"
	if strings.TrimSpace(bin) == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "emailops-cli path is empty")
	}
	if strings.TrimSpace(dataDir) == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "EMAILOPS_DATA_DIR is empty")
	}
	return &Client{Bin: bin, DataDir: dataDir}, nil
}

// ParseEnvelope decodes one JSON envelope from stdout bytes.
func ParseEnvelope(stdout []byte) (*Envelope, error) {
	const op = "emailops.ParseEnvelope"
	stdout = bytes.TrimSpace(stdout)
	if len(stdout) == 0 {
		return nil, sirerr.New(sirerr.CodeFailed, op, "empty stdout from emailops-cli")
	}
	var env Envelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode emailops-cli json").
			With("stdout_prefix", truncate(string(stdout), 120))
	}
	return &env, nil
}

// Run executes emailops-cli with --json and optional --data-dir, returning the envelope.
func (c *Client) Run(ctx context.Context, args ...string) (*Envelope, error) {
	const op = "emailops.Client.Run"
	if c == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}

	cmdArgs := make([]string, 0, len(args)+4)
	cmdArgs = append(cmdArgs, "--json", "--data-dir", c.DataDir)
	if c.Quiet {
		cmdArgs = append(cmdArgs, "--quiet")
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, c.Bin, cmdArgs...)
	cmd.Env = mergeEnv(os.Environ(), c.ExtraEnv)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := exitCodeFrom(err, cmd)
	env, parseErr := ParseEnvelope(stdout.Bytes())
	if parseErr != nil {
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "emailops-cli failed").
				With("stderr", truncate(stderr.String(), 400)).
				With("parse", parseErr.Error())
		}
		return nil, parseErr
	}
	if !env.OK {
		return env, mapCLIFailure(env, exitCode)
	}
	if err != nil {
		return env, sirerr.Wrap(err, sirerr.CodeFailed, op, "emailops-cli exit non-zero with ok envelope")
	}
	return env, nil
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

func mapCLIFailure(env *Envelope, exitCode int) error {
	const op = "emailops.Client.Run"
	msg := "emailops-cli reported failure"
	code := sirerr.CodeFailed
	if env != nil && env.Error != nil {
		if env.Error.Message != "" {
			msg = env.Error.Message
		}
		code = mapEmailOpsCode(env.Error.Code, exitCode)
	} else {
		code = mapExitCode(exitCode)
	}
	e := sirerr.New(code, op, msg)
	if env != nil && env.Error != nil && env.Error.Code != "" {
		e = e.With("emailops_code", env.Error.Code)
	}
	return e.With("exit_code", fmt.Sprintf("%d", exitCode))
}

func mapEmailOpsCode(cliCode string, exitCode int) sirerr.Code {
	switch strings.ToLower(strings.TrimSpace(cliCode)) {
	case "invalid", "invalid_input", "bad_request":
		return sirerr.CodeInvalid
	case "not_found":
		return sirerr.CodeNotFound
	case "auth", "unauthorized", "forbidden":
		return sirerr.CodeAuth
	case "network", "sync":
		return sirerr.CodeNetwork
	case "ai", "ai_disabled", "ai_error":
		return sirerr.CodeAI
	default:
		return mapExitCode(exitCode)
	}
}

func mapExitCode(exitCode int) sirerr.Code {
	switch exitCode {
	case 2:
		return sirerr.CodeInvalid
	case 3:
		return sirerr.CodeNotFound
	case 4:
		return sirerr.CodeAuth
	case 5:
		return sirerr.CodeNetwork
	case 6:
		return sirerr.CodeAI
	default:
		return sirerr.CodeFailed
	}
}

// ResolveBin finds emailops-cli: explicit path, PATH, then cargo debug under repo.
func ResolveBin(explicit, repoRoot string) (string, error) {
	const op = "emailops.ResolveBin"
	if p := strings.TrimSpace(explicit); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		return "", sirerr.New(sirerr.CodeNotFound, op, "EMAILOPS_CLI path not found").With("path", p)
	}
	if p, err := exec.LookPath("emailops-cli"); err == nil {
		return p, nil
	}
	candidates := []string{
		filepath.Join(repoRoot, "providers", "emailops", "src-tauri", "target", "debug", "emailops-cli"),
		filepath.Join(repoRoot, "providers", "emailops", "src-tauri", "target", "release", "emailops-cli"),
	}
	if cargoTarget := strings.TrimSpace(os.Getenv("CARGO_TARGET_DIR")); cargoTarget != "" {
		candidates = append([]string{
			filepath.Join(cargoTarget, "debug", "emailops-cli"),
			filepath.Join(cargoTarget, "release", "emailops-cli"),
		}, candidates...)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", sirerr.New(sirerr.CodeUnavailable, op,
		"emailops-cli not found; run make emailops-cli or set EMAILOPS_CLI")
}

// EnsureBin resolves the binary, building via cargo when missing under repoRoot.
func EnsureBin(ctx context.Context, explicit, repoRoot string) (string, error) {
	const op = "emailops.EnsureBin"
	if bin, err := ResolveBin(explicit, repoRoot); err == nil {
		return bin, nil
	}
	if strings.TrimSpace(repoRoot) == "" {
		return "", sirerr.New(sirerr.CodeUnavailable, op, "repo root unknown; cannot build emailops-cli")
	}
	targetDir := filepath.Join(repoRoot, "providers", "emailops", "src-tauri", "target")
	if err := buildCLI(ctx, repoRoot, targetDir); err != nil {
		return "", err
	}
	bin, err := ResolveBin(explicit, repoRoot)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeUnavailable, op, "emailops-cli still missing after build")
	}
	return bin, nil
}

func buildCLI(ctx context.Context, repoRoot, targetDir string) error {
	const op = "emailops.buildCLI"
	dir := filepath.Join(repoRoot, "providers", "emailops")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return sirerr.New(sirerr.CodeUnavailable, op, "providers/emailops missing; run make emailops-submodule")
	}
	cmd := exec.CommandContext(ctx, "cargo", "build",
		"--manifest-path", "src-tauri/Cargo.toml",
		"--target-dir", targetDir,
		"--no-default-features",
		"--features", "cli",
		"--bin", "emailops-cli",
	)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, op, "cargo build emailops-cli failed")
	}
	return nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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

// LaunchUI starts the EmailOps desktop app via make dev (or npm run tauri dev)
// in repoPath, with ExtraEnv injected. Blocks until the process exits.
func LaunchUI(ctx context.Context, repoPath string, extraEnv []string) error {
	const op = "emailops.LaunchUI"
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	dir := strings.TrimSpace(repoPath)
	if dir == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "emailops repo path is empty")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return sirerr.New(sirerr.CodeUnavailable, op, "emailops repo path missing").With("path", dir)
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("make"); err == nil {
		// dev is the EmailOps Makefile target for the desktop app (npm run tauri dev).
		cmd = exec.CommandContext(ctx, "make", "dev")
	} else {
		cmd = exec.CommandContext(ctx, "npm", "run", "tauri", "dev")
	}
	cmd.Dir = dir
	cmd.Env = mergeEnv(os.Environ(), extraEnv)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "launch EmailOps UI failed").With("dir", dir)
	}
	return nil
}
