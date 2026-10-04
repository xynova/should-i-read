package config

import (
	"os"
	"strings"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
	"gopkg.in/yaml.v3"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const polypusPlaceholder = "${POLYPUS_BASE_URL}"

// BumpOperatorConfig patches the live operator config toward the current init template
// without a full --force overwrite. Safe to run repeatedly.
func BumpOperatorConfig() (path string, changed bool, notes []string, err error) {
	const op = "config.BumpOperatorConfig"
	path, err = ResolvePath("")
	if err != nil {
		return "", false, nil, err
	}
	if path == "" {
		return "", false, nil, sirerr.New(sirerr.CodeNotFound, op, "config missing; run should-i-read init")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false, nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "read config").With("path", path)
	}
	var file File
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return "", false, nil, sirerr.Wrap(err, sirerr.CodeInvalid, op, "parse config yaml").With("path", path)
	}

	changed = false
	if !secretDeclared(file.Secrets, envPolypusBaseURL) {
		file.Secrets = append(file.Secrets, operatorconfig.Secret{Env: envPolypusBaseURL})
		changed = true
		notes = append(notes, "added "+envPolypusBaseURL+" to secrets:")
	}
	if shouldBumpPolypusBaseURL(file.Polypus.BaseURL) {
		file.Polypus.BaseURL = polypusPlaceholder
		changed = true
		notes = append(notes, "polypus.base_url → "+polypusPlaceholder)
	}
	if oauthChanged, oauthNotes := bumpMigrateLegacyEmailOps(&file, hadYAMLKey(raw, "emailops")); oauthChanged {
		changed = true
		notes = append(notes, oauthNotes...)
	}
	if !changed {
		return path, false, notes, nil
	}
	out, err := yaml.Marshal(fileForWrite(file))
	if err != nil {
		return "", false, nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "marshal config yaml")
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return "", false, nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "write config").With("path", path)
	}
	return path, true, notes, nil
}

func secretDeclared(secrets []operatorconfig.Secret, env string) bool {
	for _, s := range secrets {
		if strings.TrimSpace(s.Env) == env {
			return true
		}
	}
	return false
}

type fileWrite struct {
	Secrets  []operatorconfig.Secret `yaml:"secrets"`
	Polypus  PolypusFile             `yaml:"polypus"`
	OAuth    OAuthFile               `yaml:"oauth,omitempty"`
	Pimalaya PimalayaFile            `yaml:"pimalaya"`
}

func fileForWrite(f File) fileWrite {
	return fileWrite{
		Secrets:  f.Secrets,
		Polypus:  f.Polypus,
		OAuth:    f.OAuth,
		Pimalaya: f.Pimalaya,
	}
}

func hadYAMLKey(raw []byte, key string) bool {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return false
	}
	_, ok := doc[key]
	return ok
}

func oauthFieldsEmpty(o OAuthFile) bool {
	return strings.TrimSpace(o.GmailClientID) == "" &&
		strings.TrimSpace(o.GmailClientSecret) == "" &&
		strings.TrimSpace(o.OutlookClientID) == ""
}

func legacyOAuthNonEmpty(o OAuthFile) bool {
	return !oauthFieldsEmpty(o)
}

// bumpMigrateLegacyEmailOps moves oauth client fields out of deprecated emailops: and drops the section on write.
func bumpMigrateLegacyEmailOps(file *File, hadEmailOpsKey bool) (bool, []string) {
	if !hadEmailOpsKey {
		return false, nil
	}
	var notes []string
	changed := false
	legacy := file.EmailOpsLegacy
	if oauthFieldsEmpty(file.OAuth) && legacyOAuthNonEmpty(legacy) {
		file.OAuth = mergeOAuthFields(*file)
		notes = append(notes, "emailops oauth fields → oauth:")
		changed = true
	} else if legacyOAuthNonEmpty(legacy) {
		notes = append(notes, "removed deprecated emailops: (oauth: already set)")
		changed = true
	} else {
		notes = append(notes, "removed deprecated emailops: (data_dir, cli_path, repo_path)")
		changed = true
	}
	file.EmailOpsLegacy = OAuthFile{}
	return changed, notes
}

func shouldBumpPolypusBaseURL(baseURL string) bool {
	s := strings.TrimSpace(baseURL)
	if s == polypusPlaceholder {
		return false
	}
	if strings.Contains(s, "${") {
		return false
	}
	return s == "" || s == defaultPolypusBaseURL
}
