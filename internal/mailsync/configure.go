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
	Provider           Provider
	Account            string
	Email              string
	StoreRoot          string
	Force              bool
	RepoRoot           string
	HostBin            string
	SkipInit           bool
	InitReplica        ReplicaInit
	ExcludeCollections []string
}

// ReplicaInit initializes the pimdir replica (typically neverest init).
type ReplicaInit func(ctx context.Context, cfg config.Config, account string) error

// Result is returned after configure completes.
type Result struct {
	Provider          string   `json:"provider"`
	Account           string   `json:"account"`
	Email             string   `json:"email"`
	MailSyncConfig    string   `json:"mail_sync_config"`
	PimdirPath        string   `json:"pimdir_path"`
	HostConfigPath    string   `json:"host_config_path"`
	ConfigWritten     bool     `json:"config_written"`
	HostConfigPatched bool     `json:"host_config_patched"`
	HimalayaConfig    string   `json:"himalaya_config"`
	HimalayaWritten   bool     `json:"himalaya_written"`
	InitSkipped       bool     `json:"init_skipped"`
	AlreadyConfigured bool     `json:"already_configured"`
	NextSteps         []string `json:"next_steps"`
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
	dest, err := resolveConfigureDest(opts, s.hostBin)
	if err != nil {
		return Result{}, err
	}
	body, himalayaBody, err := renderConfigureBodies(opts, dest)
	if err != nil {
		return Result{}, err
	}
	res := Result{
		Provider:       string(opts.Provider),
		Account:        dest.account,
		Email:          dest.email,
		MailSyncConfig: dest.mailSyncPath,
		HimalayaConfig: dest.himalayaPath,
		PimdirPath:     dest.storeRoot,
	}
	if err := writeMailSyncIfNeeded(dest.mailSyncPath, body, opts.Force, &res); err != nil {
		return Result{}, err
	}
	written, err := writeConfigFileIfNeeded(ctx, dest.himalayaPath, himalayaBody, opts.Force, op)
	if err != nil {
		return res, err
	}
	if written {
		res.HimalayaWritten = true
	}
	hostPath, patched, err := config.PimalayaPatch(dest.mailSyncPath, dest.storeRoot, dest.account, dest.himalayaPath)
	if err != nil {
		return res, err
	}
	res.HostConfigPath = hostPath
	res.HostConfigPatched = patched
	return s.maybeInitReplica(ctx, opts, dest, res)
}

type configureDest struct {
	email, account, storeRoot, mailSyncPath, himalayaPath, hostBin string
}

func resolveConfigureDest(opts Options, serviceHostBin string) (configureDest, error) {
	const op = "mailsync.Configure"
	email := strings.TrimSpace(opts.Email)
	if email == "" {
		return configureDest{}, sirerr.New(sirerr.CodeInvalid, op, "email is required")
	}
	account := strings.TrimSpace(opts.Account)
	if account == "" {
		account = string(opts.Provider)
	}
	mailSyncPath, err := config.DefaultMailSyncConfigPath()
	if err != nil {
		return configureDest{}, err
	}
	himalayaPath, err := config.DefaultHimalayaConfigPath()
	if err != nil {
		return configureDest{}, err
	}
	storeRoot := strings.TrimSpace(opts.StoreRoot)
	if storeRoot == "" {
		storeRoot = config.DefaultPimStoreDir(account)
	}
	if storeRoot == "" {
		return configureDest{}, sirerr.New(sirerr.CodeFailed, op, "could not resolve pimdir store path")
	}
	hostBin := strings.TrimSpace(opts.HostBin)
	if hostBin == "" {
		hostBin = serviceHostBin
	}
	return configureDest{
		email: email, account: account, storeRoot: storeRoot,
		mailSyncPath: mailSyncPath, himalayaPath: himalayaPath, hostBin: hostBin,
	}, nil
}

func renderConfigureBodies(opts Options, dest configureDest) (body, himalayaBody string, err error) {
	tokenArgv, err := formatTokenCommandTOML(buildTokenArgv(dest.hostBin, opts.Provider, dest.account))
	if err != nil {
		return "", "", err
	}
	excludes := opts.ExcludeCollections
	if excludes == nil {
		excludes = config.DefaultSyncExcludeCollections()
	}
	body, err = renderAccountTOML(accountTemplateData{
		Account:            dest.account,
		IMAPServer:         opts.Provider.imapServer(),
		Email:              dest.email,
		StoreRoot:          dest.storeRoot,
		TokenCommandTOML:   tokenArgv,
		ExcludeCollections: excludes,
	})
	if err != nil {
		return "", "", err
	}
	himalayaBody, err = renderHimalayaTOML(dest.account, dest.storeRoot, opts.Provider.trashMailbox())
	return body, himalayaBody, err
}

func writeMailSyncIfNeeded(path, body string, force bool, res *Result) error {
	const op = "mailsync.Configure"
	existing, readErr := os.ReadFile(path)
	if readErr == nil && !force {
		if string(existing) == body {
			res.AlreadyConfigured = true
			return nil
		}
		return sirerr.New(sirerr.CodeFailed, op, "mail sync config exists; re-run with --force or remove file").
			With("path", path)
	}
	if readErr != nil && !os.IsNotExist(readErr) {
		return sirerr.Wrap(readErr, sirerr.CodeFailed, op, "read mail sync config").With("path", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "create config dir")
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write mail sync config").With("path", path)
	}
	res.ConfigWritten = true
	return nil
}

func (s *Service) maybeInitReplica(ctx context.Context, opts Options, dest configureDest, res Result) (Result, error) {
	if opts.SkipInit || !oauthTokenReady(opts.Provider, dest.account) {
		res.InitSkipped = true
		res.NextSteps = nextSteps(opts.Provider, dest.account)
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
	if err := initFn(ctx, cfg, dest.account); err != nil {
		return res, &ConfigureError{Result: res, Err: err}
	}
	res.NextSteps = nextSteps(opts.Provider, dest.account)
	return res, nil
}

func nextSteps(provider Provider, account string) []string {
	return []string{
		"should-i-read mail setup",
		"go tool task polypus-check",
		"go tool task mail:readiness && go tool task mail:sync",
	}
}

func writeConfigFileIfNeeded(ctx context.Context, path, body string, force bool, op string) (written bool, err error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	existing, readErr := os.ReadFile(path)
	if readErr == nil && !force {
		if string(existing) == body {
			return false, nil
		}
		return false, sirerr.New(sirerr.CodeFailed, op, "config exists; re-run with --force or remove file").
			With("path", path)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return false, sirerr.Wrap(readErr, sirerr.CodeFailed, op, "read config file").With("path", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, sirerr.Wrap(err, sirerr.CodeFailed, op, "create config dir")
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return false, sirerr.Wrap(err, sirerr.CodeFailed, op, "write config file").With("path", path)
	}
	return true, nil
}

func defaultReplicaInit(ctx context.Context, cfg config.Config, account string) error {
	nev, err := pimalaya.CreateNeverest(cfg)
	if err != nil {
		return err
	}
	_, err = nev.Init(ctx, account)
	return err
}
