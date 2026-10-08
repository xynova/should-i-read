package mailsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// ConfigureError carries a partial result when configure wrote files but a later step failed.
type ConfigureError struct {
	Result Result
	Err    error
}

func (e *ConfigureError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ConfigureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Options drives pim configure.
type Options struct {
	Provider   Provider
	Account    string
	Email      string
	StoreRoot  string
	Force      bool
	RepoRoot   string
	HostBin    string
	SkipInit   bool
	InitReplica ReplicaInit
}

// ReplicaInit initializes the pimdir replica (typically neverest init).
type ReplicaInit func(ctx context.Context, cfg config.Config, account string) error

// Result is returned after configure completes.
type Result struct {
	Provider         string `json:"provider"`
	Account          string `json:"account"`
	Email            string `json:"email"`
	MailSyncConfig   string `json:"mail_sync_config"`
	PimdirPath       string `json:"pimdir_path"`
	HostConfigPath   string `json:"host_config_path"`
	ConfigWritten    bool   `json:"config_written"`
	HostConfigPatched bool  `json:"host_config_patched"`
	InitSkipped      bool   `json:"init_skipped"`
	AlreadyConfigured bool  `json:"already_configured"`
	NextSteps        []string `json:"next_steps"`
}

// Service runs mail sync configuration.
type Service struct {
	repoRoot string
	hostBin  string
	initFn   ReplicaInit
}

// Create builds a Service from Options fields used for defaults.
func Create(repoRoot, hostBin string, initFn ReplicaInit) (*Service, error) {
	const op = "mailsync.Create"
	if strings.TrimSpace(repoRoot) == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "repo root is empty")
	}
	if initFn == nil {
		initFn = defaultReplicaInit
	}
	bin := strings.TrimSpace(hostBin)
	if bin == "" {
		if exe, err := os.Executable(); err == nil {
			bin = exe
		}
	}
	return &Service{repoRoot: repoRoot, hostBin: bin, initFn: initFn}, nil
}

// Configure writes mail-sync.toml, patches host YAML, and initializes the replica.
func (s *Service) Configure(ctx context.Context, opts Options) (Result, error) {
	const op = "mailsync.Configure"
	if s == nil {
		return Result{}, sirerr.New(sirerr.CodeInvalid, op, "nil service")
	}
	if ctx == nil {
		return Result{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if opts.Provider == "" {
		return Result{}, sirerr.New(sirerr.CodeInvalid, op, "provider is required")
	}
	email := strings.TrimSpace(opts.Email)
	if email == "" {
		return Result{}, sirerr.New(sirerr.CodeInvalid, op, "email is required")
	}
	account := strings.TrimSpace(opts.Account)
	if account == "" {
		account = string(opts.Provider)
	}
	mailSyncPath, err := config.DefaultMailSyncConfigPath()
	if err != nil {
		return Result{}, err
	}
	storeRoot := strings.TrimSpace(opts.StoreRoot)
	if storeRoot == "" {
		storeRoot = config.DefaultPimStoreDir(account)
	}
	if storeRoot == "" {
		return Result{}, sirerr.New(sirerr.CodeFailed, op, "could not resolve pimdir store path")
	}

	hostBin := strings.TrimSpace(opts.HostBin)
	if hostBin == "" {
		hostBin = s.hostBin
	}
	tokenArgv, err := formatTokenCommandTOML(buildTokenArgv(hostBin, opts.Provider, account))
	if err != nil {
		return Result{}, err
	}
	body, err := renderAccountTOML(accountTemplateData{
		Account:          account,
		IMAPServer:       opts.Provider.imapServer(),
		Email:            email,
		StoreRoot:        storeRoot,
		TokenCommandTOML: tokenArgv,
	})
	if err != nil {
		return Result{}, err
	}

	res := Result{
		Provider:       string(opts.Provider),
		Account:        account,
		Email:          email,
		MailSyncConfig: mailSyncPath,
		PimdirPath:     storeRoot,
	}

	existing, readErr := os.ReadFile(mailSyncPath)
	if readErr == nil && !opts.Force {
		if string(existing) == body {
			res.AlreadyConfigured = true
		} else {
			return Result{}, sirerr.New(sirerr.CodeFailed, op, "mail sync config exists; re-run with --force or remove file").
				With("path", mailSyncPath)
		}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return Result{}, sirerr.Wrap(readErr, sirerr.CodeFailed, op, "read mail sync config").With("path", mailSyncPath)
	}

	if !res.AlreadyConfigured {
		if err := os.MkdirAll(filepath.Dir(mailSyncPath), 0o700); err != nil {
			return Result{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "create config dir")
		}
		if err := os.WriteFile(mailSyncPath, []byte(body), 0o600); err != nil {
			return Result{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "write mail sync config").With("path", mailSyncPath)
		}
		res.ConfigWritten = true
	}

	hostPath, patched, err := config.PimalayaPatch(mailSyncPath, storeRoot, account)
	if err != nil {
		return res, err
	}
	res.HostConfigPath = hostPath
	res.HostConfigPatched = patched

	if opts.SkipInit {
		res.InitSkipped = true
		res.NextSteps = nextSteps(opts.Provider, account)
		return res, nil
	}
	if !oauthTokenReady(opts.Provider, account) {
		res.InitSkipped = true
		res.NextSteps = nextSteps(opts.Provider, account)
		return res, nil
	}
	initFn := opts.InitReplica
	if initFn == nil {
		initFn = s.initFn
	}
	cfg, err := config.Load(s.repoRoot, "")
	if err != nil {
		return res, err
	}
	if err := initFn(ctx, cfg, account); err != nil {
		return res, &ConfigureError{Result: res, Err: err}
	}
	res.NextSteps = nextSteps(opts.Provider, account)
	return res, nil
}

func nextSteps(provider Provider, account string) []string {
	return []string{
		"should-i-read mail setup",
		"make polypus-check",
		"make readiness && make sync",
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
