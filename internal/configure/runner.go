package configure

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/mailsync"
	"github.com/xynova/should-i-read/internal/oauthcred"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/setup"
	"github.com/xynova/should-i-read/internal/sirerr"
	"github.com/xynova/should-i-read/internal/token"
)

// Hooks injects dependencies for tests.
type Hooks struct {
	EnsureDep       mailsync.EnsureDep
	InitReplica     mailsync.ReplicaInit
	MailboxLogin    mailsync.MailboxLogin
	TokenReady      func(mailsync.Provider, string) bool
	RunOAuthClients func(ctx context.Context, interactive bool, out, errW io.Writer) (setup.Result, error)
}

// Runner executes configure steps.
type Runner struct {
	repoRoot string
	hostBin  string
	hooks    Hooks
}

// CreateRunner builds a Runner with production defaults.
func CreateRunner(repoRoot, hostBin string, hooks Hooks) (*Runner, error) {
	const op = "configure.CreateRunner"
	if strings.TrimSpace(repoRoot) == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "repo root is empty")
	}
	bin := strings.TrimSpace(hostBin)
	if bin == "" {
		if exe, err := os.Executable(); err == nil {
			bin = exe
		}
	}
	if hooks.EnsureDep == nil {
		hooks.EnsureDep = defaultEnsureDep(repoRoot)
	}
	if hooks.InitReplica == nil {
		hooks.InitReplica = defaultReplicaInit
	}
	if hooks.MailboxLogin == nil {
		hooks.MailboxLogin = defaultMailboxLogin
	}
	if hooks.RunOAuthClients == nil {
		hooks.RunOAuthClients = defaultRunOAuthClients(repoRoot)
	}
	return &Runner{repoRoot: repoRoot, hostBin: bin, hooks: hooks}, nil
}

func (r *Runner) tokenReady(provider mailsync.Provider, account string) bool {
	if r != nil && r.hooks.TokenReady != nil {
		return r.hooks.TokenReady(provider, account)
	}
	return oauthTokenReady(provider, account)
}

func oauthTokenReady(provider mailsync.Provider, account string) bool {
	label := strings.TrimSpace(account)
	if label == "" {
		label = string(provider)
	}
	switch provider {
	case mailsync.ProviderGmail:
		_, err := token.LoadGmailToken(label)
		return err == nil
	case mailsync.ProviderOutlook:
		_, err := token.LoadOutlookMSALCache(label)
		return err == nil
	default:
		return false
	}
}

// RunStep executes one configure step. ForceRun runs even when probe is OK.
func (r *Runner) RunStep(ctx context.Context, stepID string, opts Options, out, errW io.Writer, forceRun bool) (StepResult, config.Config, error) {
	const op = "configure.RunStep"
	if r == nil {
		return StepResult{}, config.Config{}, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	if ctx == nil {
		return StepResult{}, config.Config{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	cfg, err := config.Load(r.repoRoot, "")
	if err != nil {
		return StepResult{}, config.Config{}, err
	}
	snap, err := CollectSnapshot(ctx, r.repoRoot, cfg)
	if err != nil {
		return StepResult{}, cfg, err
	}
	if !forceRun && stepOK(snap, stepID) {
		return StepResult{Step: stepID, OK: true, Skipped: true, Detail: "Already complete."}, cfg, nil
	}
	res, cfg, err := r.runStepBody(ctx, stepID, opts, out, errW, cfg)
	return res, cfg, err
}

func (r *Runner) runStepBody(ctx context.Context, stepID string, opts Options, out, errW io.Writer, cfg config.Config) (StepResult, config.Config, error) {
	const op = "configure.runStepBody"
	res := StepResult{Step: stepID}
	switch stepID {
	case StepHostConfig:
		example, err := config.ExampleYAML()
		if err != nil {
			return res, cfg, err
		}
		_, created, err := config.WriteInit(false, example)
		if err != nil {
			return res, cfg, sirerr.Wrap(err, sirerr.CodeFailed, op, "write operator config")
		}
		res.OK = true
		if created {
			res.Detail = "Created operator config."
		} else {
			res.Detail = "Operator config is ready."
		}
		cfg, err = config.Load(r.repoRoot, "")
		return res, cfg, err
	case StepOAuthClients:
		setupRes, err := r.hooks.RunOAuthClients(ctx, opts.Interactive, out, errW)
		if err != nil {
			res.Detail = "OAuth app credential setup failed."
			return res, cfg, err
		}
		_ = setupRes
		res.OK = true
		res.Detail = "OAuth app credentials step finished."
		return res, cfg, nil
	case StepMailDependency:
		if err := r.hooks.EnsureDep(ctx, true); err != nil {
			res.Detail = "Could not install or verify the mail sync dependency."
			return res, cfg, err
		}
		res.OK = true
		res.Detail = "Mail sync dependency is available."
		cfg, err := config.Load(r.repoRoot, "")
		return res, cfg, err
	case StepMailboxProfile:
		prov, account, email, err := r.mailFields(opts, cfg)
		if err != nil {
			return res, cfg, err
		}
		svc, err := mailsync.Create(r.repoRoot, r.hostBin, r.hooks.InitReplica)
		if err != nil {
			return res, cfg, err
		}
		_, err = svc.Configure(ctx, mailsync.Options{
			Provider:  prov,
			Account:   account,
			Email:     email,
			StoreRoot: strings.TrimSpace(opts.StoreRoot),
			Force:     opts.Force,
			HostBin:   r.hostBin,
			SkipInit:  true,
		})
		if err != nil {
			res.Detail = "Could not save mailbox settings."
			return res, cfg, err
		}
		res.OK = true
		res.Detail = "Mailbox settings saved."
		cfg, err = config.Load(r.repoRoot, "")
		return res, cfg, err
	case StepMailboxLogin:
		prov, account, _, err := r.mailFields(opts, cfg)
		if err != nil {
			return res, cfg, err
		}
		if !opts.Interactive {
			res.Detail = "Mailbox login requires an interactive terminal."
			return res, cfg, sirerr.New(sirerr.CodeInvalid, op, res.Detail)
		}
		if err := r.hooks.MailboxLogin(ctx, cfg, prov, account); err != nil {
			res.Detail = "Mailbox login failed."
			return res, cfg, err
		}
		res.OK = true
		res.Detail = "Mailbox login stored."
		cfg, err = config.Load(r.repoRoot, "")
		return res, cfg, err
	case StepLocalStore:
		prov, account, _, err := r.mailFields(opts, cfg)
		if err != nil {
			return res, cfg, err
		}
		if !r.tokenReady(prov, account) {
			res.Detail = "Complete mailbox login before initializing the local store."
			return res, cfg, sirerr.New(sirerr.CodeInvalid, op, res.Detail)
		}
		mailSyncPath := strings.TrimSpace(cfg.Pimalaya.NeverestConfig)
		if mailSyncPath == "" {
			p, err := config.DefaultMailSyncConfigPath()
			if err != nil {
				return res, cfg, err
			}
			mailSyncPath = p
		}
		if err := mailsync.EnsureMailSyncTokenCommand(mailSyncPath, r.hostBin); err != nil {
			res.Detail = "Could not repair mail sync token command."
			return res, cfg, sirerr.Wrap(err, sirerr.CodeFailed, op, res.Detail)
		}
		if err := r.hooks.InitReplica(ctx, cfg, account); err != nil {
			res.Detail = "Local mail store setup failed."
			return res, cfg, sirerr.Wrap(err, sirerr.CodeFailed, op, res.Detail)
		}
		res.OK = true
		res.Detail = "Local mail store is ready."
		cfg, err = config.Load(r.repoRoot, "")
		return res, cfg, err
	default:
		return res, cfg, sirerr.New(sirerr.CodeInvalid, op, "unknown step").With("step", stepID)
	}
}

func (r *Runner) mailFields(opts Options, cfg config.Config) (mailsync.Provider, string, string, error) {
	const op = "configure.mailFields"
	email := strings.TrimSpace(opts.Email)
	account := strings.TrimSpace(opts.Account)
	providerRaw := strings.TrimSpace(opts.Provider)
	if email == "" && cfg.Pimalaya.NeverestConfig != "" {
		if raw, err := os.ReadFile(cfg.Pimalaya.NeverestConfig); err == nil {
			email = mailsync.EmailFromMailSyncTOML(string(raw))
			if providerRaw == "" {
				providerRaw = string(mailsync.InferProviderFromMailSyncTOML(string(raw)))
			}
			if account == "" {
				account = mailsync.AccountFromMailSyncTOML(string(raw))
			}
		}
	}
	if account == "" {
		account = strings.TrimSpace(cfg.Pimalaya.DefaultAccount)
	}
	if providerRaw == "" {
		providerRaw = inferProviderFromAccount(account)
	}
	prov, err := mailsync.ParseProvider(providerRaw)
	if err != nil {
		return "", "", "", sirerr.Wrap(err, sirerr.CodeInvalid, op, "provider required")
	}
	if email == "" {
		return prov, account, "", sirerr.New(sirerr.CodeInvalid, op, "email required for this step")
	}
	if account == "" {
		account = string(prov)
	}
	return prov, account, email, nil
}

func inferProviderFromAccount(account string) string {
	switch strings.ToLower(strings.TrimSpace(account)) {
	case "gmail":
		return "gmail"
	case "outlook":
		return "outlook"
	default:
		return ""
	}
}

// RunAllMissing runs incomplete steps in order.
func (r *Runner) RunAllMissing(ctx context.Context, opts Options, out, errW io.Writer, confirmLogin func(ctx context.Context) (bool, error)) (SessionResult, error) {
	const op = "configure.RunAllMissing"
	if ctx == nil {
		return SessionResult{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	session := SessionResult{Ran: []StepResult{}}
	cfg, err := config.Load(r.repoRoot, "")
	if err != nil {
		return session, err
	}
	for _, stepID := range StepOrder {
		snap, err := CollectSnapshot(ctx, r.repoRoot, cfg)
		if err != nil {
			return session, err
		}
		session.Snapshot = snap
		if stepOK(snap, stepID) {
			continue
		}
		if opts.SkipLogin && (stepID == StepMailboxLogin || stepID == StepLocalStore) {
			session.Snapshot.WaitingOn = WaitingOnMailboxLogin
			return session, nil
		}
		if stepID == StepMailboxLogin && opts.Interactive && confirmLogin != nil {
			ok, err := confirmLogin(ctx)
			if err != nil {
				return session, err
			}
			if !ok {
				session.Snapshot.WaitingOn = WaitingOnMailboxLogin
				return session, nil
			}
		}
		if needsMailFields(stepID) {
			if _, _, _, err := r.mailFields(opts, cfg); err != nil {
				return session, err
			}
		}
		stepRes, newCfg, err := r.RunStep(ctx, stepID, opts, out, errW, false)
		session.Ran = append(session.Ran, stepRes)
		cfg = newCfg
		if err != nil {
			snap, _ := CollectSnapshot(ctx, r.repoRoot, cfg)
			session.Snapshot = snap
			return session, err
		}
	}
	snap, err := CollectSnapshot(ctx, r.repoRoot, cfg)
	if err != nil {
		return session, err
	}
	session.Snapshot = snap
	return session, nil
}

func needsMailFields(stepID string) bool {
	switch stepID {
	case StepMailboxProfile, StepMailboxLogin, StepLocalStore:
		return true
	default:
		return false
	}
}

func defaultReplicaInit(ctx context.Context, cfg config.Config, account string) error {
	nev, err := pimalaya.CreateNeverest(cfg)
	if err != nil {
		return err
	}
	_, err = nev.Init(ctx, account)
	return err
}

func defaultMailboxLogin(ctx context.Context, cfg config.Config, provider mailsync.Provider, account string) error {
	switch provider {
	case mailsync.ProviderGmail:
		return token.CreateGmailBroker(cfg, account).Login(ctx)
	case mailsync.ProviderOutlook:
		return token.CreateOutlookBroker(cfg, account).Login(ctx)
	default:
		return sirerr.New(sirerr.CodeInvalid, "configure.MailboxLogin", "unsupported provider")
	}
}

func defaultRunOAuthClients(repoRoot string) func(ctx context.Context, interactive bool, out, errW io.Writer) (setup.Result, error) {
	return func(ctx context.Context, interactive bool, out, errW io.Writer) (setup.Result, error) {
		cfg := setup.Config{
			RepoRoot: repoRoot,
			Out:      out,
			Err:      errW,
		}
		if interactive {
			cfg.Prompter = setup.HuhPrompter{}
		}
		wiz, err := setup.Create(cfg)
		if err != nil {
			return setup.Result{}, err
		}
		if !interactive && !oauthcred.HasAny() {
			return setup.Result{}, sirerr.New(sirerr.CodeNotFound, "configure.oauth", "OAuth app credentials missing; run configure on a TTY")
		}
		return wiz.Run(ctx)
	}
}

func defaultEnsureDep(repoRoot string) mailsync.EnsureDep {
	return func(ctx context.Context, installIfMissing bool) error {
		check := filepath.Join(repoRoot, "scripts", "check-neverest.sh")
		if err := runScript(ctx, check); err == nil {
			return nil
		}
		if !installIfMissing {
			return sirerr.New(sirerr.CodeUnavailable, "configure.EnsureDep", "mail sync dependency missing")
		}
		install := filepath.Join(repoRoot, "scripts", "install-neverest.sh")
		return runScript(ctx, install)
	}
}

func runScript(ctx context.Context, path string) error {
	c := exec.CommandContext(ctx, path)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeUnavailable, "configure.runScript", "script failed")
	}
	return nil
}
