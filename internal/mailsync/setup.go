package mailsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/token"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const waitingOnMailboxLogin = "mailbox_login"

// SetupOptions drives mail setup workflow.
type SetupOptions struct {
	Provider    Provider
	Account     string
	Email       string
	StoreRoot   string
	Force       bool
	SkipLogin   bool
	HostBin     string
	Interactive bool
}

// SetupStepResult records one workflow step.
type SetupStepResult struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// SetupResult is returned from mail setup.
type SetupResult struct {
	Status    Status            `json:"status"`
	Steps     []SetupStepResult `json:"steps"`
	WaitingOn string            `json:"waiting_on,omitempty"`
}

// EnsureDep installs or verifies the mail sync dependency.
type EnsureDep func(ctx context.Context, installIfMissing bool) error

// MailboxLogin runs browser OAuth for the mailbox token.
type MailboxLogin func(ctx context.Context, cfg config.Config, provider Provider, account string) error

// WorkflowHooks injects dependencies for tests.
type WorkflowHooks struct {
	EnsureDep    EnsureDep
	MailboxLogin MailboxLogin
	InitReplica  ReplicaInit
	TokenReady   func(provider Provider, account string) bool
}

// Workflow orchestrates mail setup.
type Workflow struct {
	repoRoot string
	hostBin  string
	svc      *Service
	hooks    WorkflowHooks
}

// CreateWorkflow builds a Workflow with production defaults.
func CreateWorkflow(repoRoot, hostBin string, hooks WorkflowHooks) (*Workflow, error) {
	svc, err := Create(repoRoot, hostBin, hooks.InitReplica)
	if err != nil {
		return nil, err
	}
	if hooks.EnsureDep == nil {
		hooks.EnsureDep = defaultEnsureDep(repoRoot)
	}
	if hooks.MailboxLogin == nil {
		hooks.MailboxLogin = defaultMailboxLogin
	}
	if hooks.InitReplica == nil {
		hooks.InitReplica = defaultReplicaInit
	}
	svc.initFn = hooks.InitReplica
	return &Workflow{repoRoot: repoRoot, hostBin: svc.hostBin, svc: svc, hooks: hooks}, nil
}

// Setup runs the mailbox onboarding workflow.
func (w *Workflow) Setup(ctx context.Context, opts SetupOptions) (SetupResult, error) {
	const op = "mailsync.Setup"
	if w == nil || w.svc == nil {
		return SetupResult{}, sirerr.New(sirerr.CodeInvalid, op, "nil workflow")
	}
	if ctx == nil {
		return SetupResult{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if opts.Provider == "" {
		return SetupResult{}, sirerr.New(sirerr.CodeInvalid, op, "provider is required")
	}
	if strings.TrimSpace(opts.Email) == "" {
		return SetupResult{}, sirerr.New(sirerr.CodeInvalid, op, "email is required")
	}

	hostPath, err := config.ResolvePath("")
	if err != nil {
		return SetupResult{}, err
	}
	if hostPath == "" {
		return SetupResult{}, sirerr.New(sirerr.CodeNotFound, op, "operator config missing; run should-i-read init")
	}

	out := SetupResult{Steps: []SetupStepResult{}}

	step := SetupStepResult{ID: "ensure"}
	if err := w.hooks.EnsureDep(ctx, true); err != nil {
		step.Detail = "Could not install or verify the mail sync dependency."
		out.Steps = append(out.Steps, step)
		return out, sirerr.Wrap(err, sirerr.CodeUnavailable, op, step.Detail)
	}
	step.OK = true
	step.Detail = "Mail sync dependency is available."
	out.Steps = append(out.Steps, step)

	account := strings.TrimSpace(opts.Account)
	if account == "" {
		account = string(opts.Provider)
	}
	hostBin := strings.TrimSpace(opts.HostBin)
	if hostBin == "" {
		hostBin = w.hostBin
	}

	cfgRes, err := w.svc.Configure(ctx, Options{
		Provider:  opts.Provider,
		Account:   account,
		Email:     opts.Email,
		StoreRoot: opts.StoreRoot,
		Force:     opts.Force,
		HostBin:   hostBin,
		SkipInit:  true,
	})
	step = SetupStepResult{ID: "configure", OK: true, Detail: "Mailbox configuration is saved."}
	if err != nil {
		step.OK = false
		step.Detail = "Could not save mailbox configuration."
		out.Steps = append(out.Steps, step)
		return out, err
	}
	out.Steps = append(out.Steps, step)
	_ = cfgRes

	cfg, err := config.Load(w.repoRoot, "")
	if err != nil {
		return out, err
	}

	tokenOK := w.tokenReady(opts.Provider, account)
	step = SetupStepResult{ID: "login"}
	if tokenOK {
		step.OK = true
		step.Skipped = true
		step.Detail = "Mailbox login already stored."
		out.Steps = append(out.Steps, step)
	} else if opts.SkipLogin || !opts.Interactive {
		step.Skipped = true
		step.Detail = "Mailbox login requires an interactive session."
		out.Steps = append(out.Steps, step)
		st, err := CollectStatus(ctx, cfg)
		if err != nil {
			return out, err
		}
		out.Status = st
		out.WaitingOn = waitingOnMailboxLogin
		return out, nil
	} else {
		if err := w.hooks.MailboxLogin(ctx, cfg, opts.Provider, account); err != nil {
			step.Detail = "Mailbox login failed."
			out.Steps = append(out.Steps, step)
			return out, err
		}
		step.OK = true
		step.Detail = "Mailbox login stored."
		out.Steps = append(out.Steps, step)
		tokenOK = true
	}

	if tokenOK {
		step = SetupStepResult{ID: "init"}
		if err := w.hooks.InitReplica(ctx, cfg, account); err != nil {
			step.Detail = "Local mail store setup failed."
			out.Steps = append(out.Steps, step)
			return out, sirerr.Wrap(err, sirerr.CodeFailed, op, step.Detail)
		}
		step.OK = true
		step.Detail = "Local mail store is ready."
		out.Steps = append(out.Steps, step)
	}

	st, err := CollectStatus(ctx, cfg)
	if err != nil {
		return out, err
	}
	out.Status = st
	return out, nil
}

func (w *Workflow) tokenReady(provider Provider, account string) bool {
	if w != nil && w.hooks.TokenReady != nil {
		return w.hooks.TokenReady(provider, account)
	}
	return oauthTokenReady(provider, account)
}

func defaultEnsureDep(repoRoot string) EnsureDep {
	return func(ctx context.Context, installIfMissing bool) error {
		const op = "mailsync.EnsureDep"
		check := filepath.Join(repoRoot, "scripts", "check-neverest.sh")
		if err := runScript(ctx, check); err == nil {
			return nil
		}
		if !installIfMissing {
			return sirerr.New(sirerr.CodeUnavailable, op, "mail sync dependency missing")
		}
		install := filepath.Join(repoRoot, "scripts", "install-neverest.sh")
		return runScript(ctx, install)
	}
}

func runScript(ctx context.Context, path string) error {
	const op = "mailsync.runScript"
	c := exec.CommandContext(ctx, path)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, op, "script failed").With("script", path)
	}
	return nil
}

func defaultMailboxLogin(ctx context.Context, cfg config.Config, provider Provider, account string) error {
	switch provider {
	case ProviderGmail:
		return token.CreateGmailBroker(cfg, account).Login(ctx)
	case ProviderOutlook:
		return token.CreateOutlookBroker(cfg, account).Login(ctx)
	default:
		return sirerr.New(sirerr.CodeInvalid, "mailsync.MailboxLogin", "unsupported provider").With("provider", string(provider))
	}
}
