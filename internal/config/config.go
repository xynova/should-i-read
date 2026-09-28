package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
	"gopkg.in/yaml.v3"

	"github.com/xynova/should-i-read/internal/oauthcred"
	"github.com/xynova/should-i-read/internal/secret"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	AppName               = "should-i-read"
	defaultPolypusBaseURL = "http://127.0.0.1:1320"
	envConfigPath         = "SHOULD_I_READ_CONFIG"
)

var placeholderRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// File is the on-disk YAML shape (placeholders allowed).
type File struct {
	Secrets  []operatorconfig.Secret `yaml:"secrets"`
	Polypus  PolypusFile             `yaml:"polypus"`
	EmailOps EmailOpsFile            `yaml:"emailops"`
}

// PolypusFile is the polypus YAML section.
type PolypusFile struct {
	BaseURL string `yaml:"base_url"`
}

// EmailOpsFile is the emailops YAML section.
type EmailOpsFile struct {
	DataDir           string `yaml:"data_dir"`
	CLIPath           string `yaml:"cli_path"`
	RepoPath          string `yaml:"repo_path"`
	DefaultAccount    string `yaml:"default_account"`
	GmailClientID     string `yaml:"gmail_client_id"`
	GmailClientSecret string `yaml:"gmail_client_secret"`
	OutlookClientID   string `yaml:"outlook_client_id"`
}

// Config is the resolved runtime config used by the CLI.
type Config struct {
	Path              string
	RepoRoot          string
	PolypusBaseURL    string
	EmailOpsDataDir   string
	EmailOpsCLI       string
	EmailOpsRepoPath  string
	DefaultAccount    string
	GmailClientID     string
	GmailClientSecret string
	OutlookClientID   string
}

// ChildEnv returns env vars to inject into EmailOps child processes.
func (c Config) ChildEnv() []string {
	if c.EmailOpsDataDir == "" && c.GmailClientID == "" && c.GmailClientSecret == "" && c.OutlookClientID == "" {
		return nil
	}
	out := make([]string, 0, 8)
	if c.EmailOpsDataDir != "" {
		out = append(out, "EMAILOPS_DATA_DIR="+c.EmailOpsDataDir)
	}
	if c.GmailClientID != "" {
		out = append(out, "EMAILOPS_GMAIL_CLIENT_ID="+c.GmailClientID)
	}
	if c.GmailClientSecret != "" {
		out = append(out, "EMAILOPS_GMAIL_CLIENT_SECRET="+c.GmailClientSecret)
	}
	if c.OutlookClientID != "" {
		out = append(out, "EMAILOPS_OUTLOOK_CLIENT_ID="+c.OutlookClientID)
	}
	return out
}

// Redacted returns a copy safe to print (secrets as set/unset).
func (c Config) Redacted() map[string]any {
	return map[string]any{
		"path": c.Path,
		"polypus": map[string]string{
			"base_url": c.PolypusBaseURL,
		},
		"emailops": map[string]string{
			"data_dir":            c.EmailOpsDataDir,
			"cli_path":            c.EmailOpsCLI,
			"repo_path":           c.EmailOpsRepoPath,
			"default_account":     c.DefaultAccount,
			"gmail_client_id":     setUnset(c.GmailClientID),
			"gmail_client_secret": setUnset(c.GmailClientSecret),
			"outlook_client_id":   setUnset(c.OutlookClientID),
		},
	}
}

func setUnset(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(unset)"
	}
	return "(set)"
}

// UserConfigDir returns ~/.config/should-i-read (or XDG).
func UserConfigDir() (string, error) {
	const op = "config.UserConfigDir"
	path, err := operatorconfig.UserConfigPath(AppName, "config.yaml")
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "resolve user config path")
	}
	return filepath.Dir(path), nil
}

// UserConfigFilePath returns the live config.yaml path.
func UserConfigFilePath() (string, error) {
	return operatorconfig.UserConfigPath(AppName, "config.yaml")
}

// ResolvePath picks config file: override → SHOULD_I_READ_CONFIG → XDG user file → "".
func ResolvePath(override string) (string, error) {
	const op = "config.ResolvePath"
	path, err := operatorconfig.ResolveConfigPath(Options(override))
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeInvalid, op, "resolve config path")
	}
	return path, nil
}

// DefaultEmailOpsDataDir is the host-owned mailbox data directory.
func DefaultEmailOpsDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", AppName, "emailops")
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, AppName, "emailops")
		}
		return filepath.Join(home, "AppData", "Roaming", AppName, "emailops")
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, AppName, "emailops")
		}
		return filepath.Join(home, ".local", "share", AppName, "emailops")
	}
}

// DefaultFile returns the template written by init.
func DefaultFile() File {
	return File{
		Secrets: DefaultSecrets(),
		Polypus: PolypusFile{BaseURL: defaultPolypusBaseURL},
		EmailOps: EmailOpsFile{
			DataDir:           "",
			CLIPath:           "",
			RepoPath:          "",
			DefaultAccount:    "",
			GmailClientID:     "${EMAILOPS_GMAIL_CLIENT_ID}",
			GmailClientSecret: "${EMAILOPS_GMAIL_CLIENT_SECRET}",
			OutlookClientID:   "${EMAILOPS_OUTLOOK_CLIENT_ID}",
		},
	}
}

// ExampleYAML is the committed example document.
func ExampleYAML() ([]byte, error) {
	return yaml.Marshal(DefaultFile())
}

// Load resolves path, reads YAML (or defaults), resolves secrets, expands placeholders, applies defaults.
func Load(repoRoot, overridePath string) (Config, error) {
	return load(repoRoot, overridePath, nil)
}

func load(repoRoot, overridePath string, kr operatorconfig.Keyring) (Config, error) {
	const op = "config.Load"
	path, err := ResolvePath(overridePath)
	if err != nil {
		return Config{}, err
	}

	var file File
	if path == "" {
		file = DefaultFile()
	} else {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return Config{}, sirerr.Wrap(readErr, sirerr.CodeFailed, op, "read config file").With("path", path)
		}
		if err := yaml.Unmarshal(raw, &file); err != nil {
			return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "parse config yaml").With("path", path)
		}
	}

	opts := Options(overridePath)
	if kr != nil {
		opts.Keyring = kr
	}
	if err := ResolveHostSecrets(file.Secrets, kr); err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "resolve secrets")
	}

	cfg, err := materialize(repoRoot, path, file)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func materialize(repoRoot, path string, file File) (Config, error) {
	const op = "config.materialize"
	resolve := func(name string) (string, error) {
		return secret.Resolve(name)
	}
	oauthResolve := func(name string) (string, error) {
		return oauthcred.Resolve(name)
	}

	baseURL, err := ExpandString(file.Polypus.BaseURL, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand polypus.base_url")
	}
	dataDir, err := ExpandString(file.EmailOps.DataDir, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand emailops.data_dir")
	}
	cliPath, err := ExpandString(file.EmailOps.CLIPath, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand emailops.cli_path")
	}
	repoPath, err := ExpandString(file.EmailOps.RepoPath, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand emailops.repo_path")
	}
	account, err := ExpandString(file.EmailOps.DefaultAccount, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand emailops.default_account")
	}
	gmailID, err := ExpandString(file.EmailOps.GmailClientID, oauthResolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand emailops.gmail_client_id")
	}
	gmailSecret, err := ExpandString(file.EmailOps.GmailClientSecret, oauthResolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand emailops.gmail_client_secret")
	}
	outlookID, err := ExpandString(file.EmailOps.OutlookClientID, oauthResolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand emailops.outlook_client_id")
	}

	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultPolypusBaseURL
	}
	if strings.TrimSpace(dataDir) == "" {
		if v := strings.TrimSpace(os.Getenv("EMAILOPS_DATA_DIR")); v != "" {
			dataDir = v
		} else {
			dataDir = DefaultEmailOpsDataDir()
		}
	}
	if strings.TrimSpace(cliPath) == "" {
		cliPath = strings.TrimSpace(os.Getenv("EMAILOPS_CLI"))
	}
	if strings.TrimSpace(repoPath) == "" {
		repoPath = filepath.Join(repoRoot, "providers", "emailops")
	}

	return Config{
		Path:              path,
		RepoRoot:          repoRoot,
		PolypusBaseURL:    strings.TrimRight(baseURL, "/"),
		EmailOpsDataDir:   dataDir,
		EmailOpsCLI:       cliPath,
		EmailOpsRepoPath:  repoPath,
		DefaultAccount:    account,
		GmailClientID:     gmailID,
		GmailClientSecret: gmailSecret,
		OutlookClientID:   outlookID,
	}, nil
}

// ExpandString replaces ${VAR} using resolve. If required and any placeholder
// cannot be resolved, returns an error. If not required, unresolved placeholders
// become empty strings.
func ExpandString(s string, resolve func(string) (string, error), required bool) (string, error) {
	var firstErr error
	out := placeholderRE.ReplaceAllStringFunc(s, func(match string) string {
		m := placeholderRE.FindStringSubmatch(match)
		if len(m) != 2 {
			return ""
		}
		name := m[1]
		val, err := resolve(name)
		if err != nil || val == "" {
			if required && firstErr == nil {
				if err != nil {
					firstErr = err
				} else {
					firstErr = fmt.Errorf("unresolved placeholder ${%s}", name)
				}
			}
			return ""
		}
		return val
	})
	if required && firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

// WriteInit writes user config.yaml (if missing or force) and refreshes example.
func WriteInit(force bool, exampleSrc []byte) (configPath string, created bool, err error) {
	const op = "config.WriteInit"
	if len(exampleSrc) == 0 {
		exampleSrc, err = ExampleYAML()
		if err != nil {
			return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "marshal example yaml")
		}
	}
	created, err = operatorconfig.InitUserConfig(Options(""), exampleSrc, force)
	if err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "init user config")
	}
	configPath, err = UserConfigFilePath()
	if err != nil {
		return "", false, err
	}
	return configPath, created, nil
}
