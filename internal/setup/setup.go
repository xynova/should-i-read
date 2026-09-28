// Package setup runs the interactive mail OAuth installer.
//
// Flow: ensure config init → detect product/env credentials → BYO paste or
// guided DIY (advanced) → store client ids in Keychain when available.
// Mailbox tokens stay in EmailOps; this package never prints secret values.
package setup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/oauthcred"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Mode selects how credentials are supplied when product embeds are absent.
type Mode string

const (
	// ModeProduct means env, Keychain, or release embeds already resolve.
	ModeProduct Mode = "product"
	// ModeBYO means the operator pastes org-provided client credentials.
	ModeBYO Mode = "byo"
	// ModeGuided means open Cloud Console / Entra docs and paste afterward.
	ModeGuided Mode = "guided"
	// ModeGCloud means optional gcloud DIY (fail closed to ModeGuided).
	ModeGCloud Mode = "gcloud"
)

// Provider is a mail OAuth provider the wizard can configure.
type Provider string

const (
	ProviderGmail   Provider = "gmail"
	ProviderOutlook Provider = "outlook"
	ProviderBoth    Provider = "both"
)

// Prompter collects interactive choices. Implementations must mask secret fields.
type Prompter interface {
	SelectMode(ctx context.Context, productReady bool) (Mode, error)
	SelectProviders(ctx context.Context) (Provider, error)
	PromptGmail(ctx context.Context) (clientID, clientSecret string, err error)
	PromptOutlook(ctx context.Context) (clientID string, err error)
	Confirm(ctx context.Context, message string) (bool, error)
}

// SecretStore persists OAuth client credentials (never mailbox tokens).
type SecretStore interface {
	Set(name, value string) error
}

// Browser opens documentation URLs in the system browser.
type Browser interface {
	Open(ctx context.Context, url string) error
}

// Runner executes external commands (gcloud DIY only).
type Runner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

// Config creates a Wizard with explicit dependencies.
type Config struct {
	RepoRoot   string
	Out        io.Writer
	Err        io.Writer
	Prompter   Prompter
	Secrets    SecretStore
	Browser    Browser
	Runner     Runner
	ExampleSrc []byte
	// SkipInit skips WriteInit (tests).
	SkipInit bool
	// ForceMode skips the mode prompt when non-empty (tests / non-interactive).
	ForceMode Mode
	// ForceProviders skips the provider prompt when non-empty (tests).
	ForceProviders Provider
	// ResolveStatus overrides oauthcred.Status (tests).
	ResolveStatus func() map[string]string
	// HasProduct overrides oauthcred.HasAny (tests).
	HasProduct func() bool
}

// Wizard orchestrates setup.
type Wizard struct {
	repoRoot       string
	out            io.Writer
	errW           io.Writer
	prompter       Prompter
	secrets        SecretStore
	browser        Browser
	runner         Runner
	exampleSrc     []byte
	skipInit       bool
	forceMode      Mode
	forceProviders Provider
	resolveStatus  func() map[string]string
	hasProduct     func() bool
}

// Create builds a Wizard from Config.
func Create(cfg Config) (*Wizard, error) {
	const op = "setup.Create"
	if strings.TrimSpace(cfg.RepoRoot) == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "repo root is empty")
	}
	out := cfg.Out
	if out == nil {
		out = io.Discard
	}
	errW := cfg.Err
	if errW == nil {
		errW = io.Discard
	}
	secrets := cfg.Secrets
	if secrets == nil {
		secrets = secretStore{}
	}
	browser := cfg.Browser
	if browser == nil {
		browser = systemBrowser{}
	}
	runner := cfg.Runner
	if runner == nil {
		runner = execRunner{}
	}
	resolveStatus := cfg.ResolveStatus
	if resolveStatus == nil {
		resolveStatus = oauthcred.Status
	}
	hasProduct := cfg.HasProduct
	if hasProduct == nil {
		hasProduct = oauthcred.HasAny
	}
	return &Wizard{
		repoRoot:       cfg.RepoRoot,
		out:            out,
		errW:           errW,
		prompter:       cfg.Prompter,
		secrets:        secrets,
		browser:        browser,
		runner:         runner,
		exampleSrc:     cfg.ExampleSrc,
		skipInit:       cfg.SkipInit,
		forceMode:      cfg.ForceMode,
		forceProviders: cfg.ForceProviders,
		resolveStatus:  resolveStatus,
		hasProduct:     hasProduct,
	}, nil
}

type secretStore struct{}

func (secretStore) Set(name, value string) error {
	return config.SetSecret(name, value)
}

// Result is a redacted summary of what setup accomplished.
type Result struct {
	ConfigPath string            `json:"config_path"`
	ConfigNew  bool              `json:"config_created"`
	Mode       Mode              `json:"mode"`
	Status     map[string]string `json:"oauth_status"`
	NextSteps  []string          `json:"next_steps"`
	Notes      []string          `json:"notes,omitempty"`
}

// Run executes the setup flow.
func (w *Wizard) Run(ctx context.Context) (Result, error) {
	const op = "setup.Run"
	if ctx == nil {
		return Result{}, sirerr.New(sirerr.CodeInvalid, op, "context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}

	var (
		configPath string
		created    bool
	)
	if !w.skipInit {
		example := w.exampleSrc
		if len(example) == 0 {
			p := filepath.Join(w.repoRoot, "config", "should-i-read.example.yaml")
			if raw, err := os.ReadFile(p); err == nil {
				example = raw
			}
		}
		path, wasCreated, err := config.WriteInit(false, example)
		if err != nil {
			return Result{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "ensure config init")
		}
		configPath = path
		created = wasCreated
		dataDir := config.DefaultEmailOpsDataDir()
		if dataDir != "" {
			if err := os.MkdirAll(dataDir, 0o700); err != nil {
				return Result{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "create default data dir").With("dir", dataDir)
			}
		}
	} else {
		path, err := config.UserConfigFilePath()
		if err != nil {
			return Result{}, err
		}
		configPath = path
	}

	productReady := w.hasProduct()
	status := w.resolveStatus()
	_, _ = fmt.Fprintf(w.out, "Mail OAuth setup\n")
	_, _ = fmt.Fprintf(w.out, "  Tokens and mail stay on this computer.\n")
	_, _ = fmt.Fprintf(w.out, "  OAuth client ids only identify which app is asking for access.\n")
	_, _ = fmt.Fprintf(w.out, "  You can replace product client ids with your own (BYO).\n")
	_, _ = fmt.Fprintf(w.out, "Credential status: gmail_id=%s gmail_secret=%s outlook_id=%s\n",
		status[oauthcred.EnvGmailClientID],
		status[oauthcred.EnvGmailClientSecret],
		status[oauthcred.EnvOutlookClientID],
	)

	mode := w.forceMode
	if mode == "" {
		if productReady {
			mode = ModeProduct
			_, _ = fmt.Fprintf(w.out, "Product or env credentials already resolve. Skipping paste.\n")
		} else {
			if w.prompter == nil {
				return Result{}, sirerr.New(sirerr.CodeInvalid, op, "no credentials resolved and no prompter configured")
			}
			selected, err := w.prompter.SelectMode(ctx, false)
			if err != nil {
				return Result{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "select mode")
			}
			mode = selected
		}
	}

	res := Result{
		ConfigPath: configPath,
		ConfigNew:  created,
		Mode:       mode,
		Status:     status,
		NextSteps: []string{
			"make ui  # add each mailbox (browser consent)",
			"./bin/should-i-read accounts",
			"./bin/should-i-read doctor",
		},
	}

	switch mode {
	case ModeProduct:
		res.Notes = append(res.Notes, "Using product/env/Keychain OAuth client credentials.")
		res.Status = w.resolveStatus()
		return res, nil
	case ModeBYO:
		if err := w.runBYO(ctx, &res); err != nil {
			return res, err
		}
	case ModeGuided:
		if err := w.runGuided(ctx, &res); err != nil {
			return res, err
		}
	case ModeGCloud:
		if err := w.runGCloud(ctx, &res); err != nil {
			return res, err
		}
	default:
		return res, sirerr.New(sirerr.CodeInvalid, op, "unknown setup mode").With("mode", string(mode))
	}

	res.Status = w.resolveStatus()
	return res, nil
}

func (w *Wizard) runBYO(ctx context.Context, res *Result) error {
	const op = "setup.runBYO"
	if w.prompter == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "prompter is nil")
	}
	providers := w.forceProviders
	var err error
	if providers == "" {
		providers, err = w.prompter.SelectProviders(ctx)
		if err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "select providers")
		}
	}
	return w.storeProviders(ctx, providers, res)
}

func (w *Wizard) runGuided(ctx context.Context, res *Result) error {
	const op = "setup.runGuided"
	if w.prompter == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "prompter is nil")
	}
	providers := w.forceProviders
	var err error
	if providers == "" {
		providers, err = w.prompter.SelectProviders(ctx)
		if err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "select providers")
		}
	}
	if err := openGuidedDocs(ctx, w.browser, providers); err != nil {
		res.Notes = append(res.Notes, "Could not open browser; see docs/oauth-product-apps.md and docs/emailops-setup.md.")
	} else {
		res.Notes = append(res.Notes, "Opened guided Cloud Console / Entra documentation.")
	}
	ok, err := w.prompter.Confirm(ctx, "Paste client credentials now?")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "confirm paste")
	}
	if !ok {
		res.Notes = append(res.Notes, "Skipped paste; run make setup again after creating the OAuth app.")
		return nil
	}
	return w.storeProviders(ctx, providers, res)
}

func (w *Wizard) storeProviders(ctx context.Context, providers Provider, res *Result) error {
	const op = "setup.storeProviders"
	if err := ctx.Err(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	switch providers {
	case ProviderGmail:
		return w.storeGmail(ctx, res)
	case ProviderOutlook:
		return w.storeOutlook(ctx, res)
	case ProviderBoth:
		if err := w.storeGmail(ctx, res); err != nil {
			return err
		}
		return w.storeOutlook(ctx, res)
	default:
		return sirerr.New(sirerr.CodeInvalid, op, "unknown provider selection").With("providers", string(providers))
	}
}

func (w *Wizard) storeGmail(ctx context.Context, res *Result) error {
	const op = "setup.storeGmail"
	id, secretVal, err := w.prompter.PromptGmail(ctx)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "prompt gmail")
	}
	id = strings.TrimSpace(id)
	secretVal = strings.TrimSpace(secretVal)
	if err := ValidateGmailClientID(id); err != nil {
		return err
	}
	if err := w.secrets.Set(oauthcred.EnvGmailClientID, id); err != nil {
		return w.storeErr(err, oauthcred.EnvGmailClientID)
	}
	res.Notes = append(res.Notes, "Stored "+oauthcred.EnvGmailClientID+" (redacted).")
	if secretVal != "" {
		if err := w.secrets.Set(oauthcred.EnvGmailClientSecret, secretVal); err != nil {
			return w.storeErr(err, oauthcred.EnvGmailClientSecret)
		}
		res.Notes = append(res.Notes, "Stored "+oauthcred.EnvGmailClientSecret+" (redacted).")
	}
	return nil
}

func (w *Wizard) storeOutlook(ctx context.Context, res *Result) error {
	const op = "setup.storeOutlook"
	id, err := w.prompter.PromptOutlook(ctx)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "prompt outlook")
	}
	id = strings.TrimSpace(id)
	if err := ValidateOutlookClientID(id); err != nil {
		return err
	}
	if err := w.secrets.Set(oauthcred.EnvOutlookClientID, id); err != nil {
		return w.storeErr(err, oauthcred.EnvOutlookClientID)
	}
	res.Notes = append(res.Notes, "Stored "+oauthcred.EnvOutlookClientID+" (redacted).")
	return nil
}

// storeErr converts Keychain/unavailable failures into typed errors that never
// include secret values. On unavailable storage, prints export instructions.
func (w *Wizard) storeErr(err error, field string) error {
	const op = "setup.store"
	if err == nil {
		return nil
	}
	code, ok := sirerr.AsCode(err)
	if ok && code == sirerr.CodeUnavailable {
		PrintExportHint(w.errW, field)
		return sirerr.Wrap(err, sirerr.CodeUnavailable, op, "Keychain unavailable; export the env var instead").
			With("field", field)
	}
	return sirerr.Wrap(err, sirerr.CodeFailed, op, "store credential").With("field", field)
}

// PrintExportHint writes shell export instructions without secret values.
func PrintExportHint(w io.Writer, field string) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "Keychain unavailable. Export manually (value not echoed):\n  export %s=...\n", field)
}
