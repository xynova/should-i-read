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
	if !changed {
		return path, false, notes, nil
	}
	out, err := yaml.Marshal(&file)
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
