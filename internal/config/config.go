package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	envPolypusBaseURL     = "POLYPUS_BASE_URL"
)

var placeholderRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// File is the on-disk YAML shape (placeholders allowed).
type File struct {
	Secrets        []operatorconfig.Secret `yaml:"secrets"`
	Polypus        PolypusFile             `yaml:"polypus"`
	Taxonomy       TaxonomyFile            `yaml:"taxonomy"`
	OAuth          OAuthFile               `yaml:"oauth"`
	EmailOpsLegacy OAuthFile               `yaml:"emailops"` // deprecated; merged into oauth on load
	Pimalaya       PimalayaFile            `yaml:"pimalaya"`
}

// PolypusFile is the polypus YAML section.
type PolypusFile struct {
	BaseURL       string `yaml:"base_url"`
	ClassifyModel string `yaml:"classify_model"`
	JudgeModel    string `yaml:"judge_model"`
	EmbedModel    string `yaml:"embed_model"`
}

// PimalayaFile is the Neverest / pimdir YAML section.
type PimalayaFile struct {
	NeverestBin    string `yaml:"neverest_bin"`
	NeverestConfig string `yaml:"neverest_config"`
	DefaultAccount string `yaml:"default_account"`
	PimdirPath     string `yaml:"pimdir_path"`
}

// OAuthFile holds OAuth client ids for mail token brokers.
type OAuthFile struct {
	GmailClientID     string `yaml:"gmail_client_id"`
	GmailClientSecret string `yaml:"gmail_client_secret"`
	OutlookClientID   string `yaml:"outlook_client_id"`
}

// Config is the resolved runtime config used by the CLI.
type Config struct {
	Path                 string
	RepoRoot             string
	PolypusBaseURL       string
	PolypusClassifyModel string
	PolypusJudgeModel    string
	PolypusEmbedModel    string
	GmailClientID        string
	GmailClientSecret    string
	OutlookClientID      string
	Pimalaya             PimalayaConfig
	Taxonomy             TaxonomyConfig
}

// PimalayaConfig is resolved Neverest / pimdir settings.
type PimalayaConfig struct {
	NeverestBin    string
	NeverestConfig string
	DefaultAccount string
	PimdirPath     string
}

// OAuthEnv returns env vars for OAuth client credentials (token subprocesses).
func (c Config) OAuthEnv() []string {
	if c.GmailClientID == "" && c.GmailClientSecret == "" && c.OutlookClientID == "" {
		return nil
	}
	out := make([]string, 0, 4)
	if c.GmailClientID != "" {
		out = append(out, oauthcred.EnvGmailClientID+"="+c.GmailClientID)
	}
	if c.GmailClientSecret != "" {
		out = append(out, oauthcred.EnvGmailClientSecret+"="+c.GmailClientSecret)
	}
	if c.OutlookClientID != "" {
		out = append(out, oauthcred.EnvOutlookClientID+"="+c.OutlookClientID)
	}
	return out
}

// Redacted returns a copy safe to print (secrets as set/unset).
func (c Config) Redacted() map[string]any {
	return map[string]any{
		"path": c.Path,
		"polypus": map[string]string{
			"base_url":       c.PolypusBaseURL,
			"classify_model": setUnset(c.PolypusClassifyModel),
			"judge_model":    setUnset(c.PolypusJudgeModel),
		},
		"taxonomy": map[string]string{
			"catalog_path": c.Taxonomy.CatalogPath,
		},
		"pimalaya": map[string]string{
			"neverest_bin":    c.Pimalaya.NeverestBin,
			"neverest_config": c.Pimalaya.NeverestConfig,
			"default_account": c.Pimalaya.DefaultAccount,
			"pimdir_path":     c.Pimalaya.PimdirPath,
		},
		"oauth": map[string]string{
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

// DefaultFile returns the template written by init.
func DefaultFile() File {
	return File{
		Secrets: DefaultSecrets(),
		Polypus: PolypusFile{BaseURL: "${POLYPUS_BASE_URL}"},
		Pimalaya: PimalayaFile{
			NeverestBin:    "",
			NeverestConfig: "",
			DefaultAccount: "",
			PimdirPath:     "",
		},
		OAuth: OAuthFile{
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

func mergeOAuthFields(file File) OAuthFile {
	o := file.OAuth
	l := file.EmailOpsLegacy
	if strings.TrimSpace(o.GmailClientID) == "" {
		o.GmailClientID = l.GmailClientID
	}
	if strings.TrimSpace(o.GmailClientSecret) == "" {
		o.GmailClientSecret = l.GmailClientSecret
	}
	if strings.TrimSpace(o.OutlookClientID) == "" {
		o.OutlookClientID = l.OutlookClientID
	}
	return o
}

func materialize(repoRoot, path string, file File) (Config, error) {
	const op = "config.materialize"
	resolve := func(name string) (string, error) {
		return secret.Resolve(name)
	}
	oauthResolve := func(name string) (string, error) {
		return oauthcred.Resolve(name)
	}
	oauthFile := mergeOAuthFields(file)

	baseURL, err := ExpandString(file.Polypus.BaseURL, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand polypus.base_url")
	}
	gmailID, err := ExpandString(oauthFile.GmailClientID, oauthResolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand oauth.gmail_client_id")
	}
	gmailSecret, err := ExpandString(oauthFile.GmailClientSecret, oauthResolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand oauth.gmail_client_secret")
	}
	outlookID, err := ExpandString(oauthFile.OutlookClientID, oauthResolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand oauth.outlook_client_id")
	}
	neverestBin, err := ExpandString(file.Pimalaya.NeverestBin, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand pimalaya.neverest_bin")
	}
	neverestCfg, err := ExpandString(file.Pimalaya.NeverestConfig, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand pimalaya.neverest_config")
	}
	pimAccount, err := ExpandString(file.Pimalaya.DefaultAccount, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand pimalaya.default_account")
	}
	pimdirPath, err := ExpandString(file.Pimalaya.PimdirPath, resolve, false)
	if err != nil {
		return Config{}, sirerr.Wrap(err, sirerr.CodeInvalid, op, "expand pimalaya.pimdir_path")
	}

	if strings.TrimSpace(baseURL) == "" {
		if v := strings.TrimSpace(os.Getenv(envPolypusBaseURL)); v != "" {
			baseURL = v
		} else {
			baseURL = defaultPolypusBaseURL
		}
	}
	if strings.TrimSpace(neverestBin) == "" {
		neverestBin = strings.TrimSpace(os.Getenv("NEVEREST_BIN"))
	}
	if strings.TrimSpace(neverestCfg) == "" {
		neverestCfg = strings.TrimSpace(os.Getenv("NEVEREST_CONFIG"))
	}
	classifyModel := strings.TrimSpace(file.Polypus.ClassifyModel)
	judgeModel := strings.TrimSpace(file.Polypus.JudgeModel)
	embedModel := strings.TrimSpace(file.Polypus.EmbedModel)
	taxonomyCfg, err := resolveTaxonomy(file.Taxonomy)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Path:                 path,
		RepoRoot:             repoRoot,
		PolypusBaseURL:       strings.TrimRight(baseURL, "/"),
		PolypusClassifyModel: classifyModel,
		PolypusJudgeModel:    judgeModel,
		PolypusEmbedModel:    embedModel,
		GmailClientID:        gmailID,
		GmailClientSecret:    gmailSecret,
		OutlookClientID:      outlookID,
		Pimalaya: PimalayaConfig{
			NeverestBin:    neverestBin,
			NeverestConfig: neverestCfg,
			DefaultAccount: pimAccount,
			PimdirPath:     pimdirPath,
		},
		Taxonomy: taxonomyCfg,
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
