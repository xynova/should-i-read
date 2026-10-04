package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// PimalayaPatch updates pimalaya paths in the live operator config.yaml.
func PimalayaPatch(neverestConfig, pimdirPath, defaultAccount string) (configPath string, changed bool, err error) {
	const op = "config.PimalayaPatch"
	path, err := ResolvePath("")
	if err != nil {
		return "", false, err
	}
	if path == "" {
		return "", false, sirerr.New(sirerr.CodeNotFound, op, "config missing; run should-i-read init")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "read config").With("path", path)
	}
	var file File
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeInvalid, op, "parse config yaml").With("path", path)
	}
	changed = false
	if strings.TrimSpace(file.Pimalaya.NeverestConfig) != strings.TrimSpace(neverestConfig) {
		file.Pimalaya.NeverestConfig = neverestConfig
		changed = true
	}
	if strings.TrimSpace(file.Pimalaya.PimdirPath) != strings.TrimSpace(pimdirPath) {
		file.Pimalaya.PimdirPath = pimdirPath
		changed = true
	}
	if strings.TrimSpace(file.Pimalaya.DefaultAccount) != strings.TrimSpace(defaultAccount) {
		file.Pimalaya.DefaultAccount = defaultAccount
		changed = true
	}
	if !changed {
		return path, false, nil
	}
	out, err := yaml.Marshal(&file)
	if err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "marshal config yaml")
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return "", false, sirerr.Wrap(err, sirerr.CodeFailed, op, "write config").With("path", path)
	}
	return path, true, nil
}
