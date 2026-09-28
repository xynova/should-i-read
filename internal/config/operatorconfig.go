package config

import (
	"bytes"
	"errors"
	"os"
	"strings"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
	"gopkg.in/yaml.v3"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// ResolveHostSecrets fills env from keyring/SOPS for secrets declared in config.yaml.
// When secrets is empty, no keyring or SOPS lookups run (polypus-local pattern).
func ResolveHostSecrets(secrets []operatorconfig.Secret, kr operatorconfig.Keyring) error {
	if len(secrets) == 0 {
		return nil
	}
	opts := Options("")
	opts.Secrets = secrets
	opts.Keyring = kr
	return operatorconfig.ResolveSecrets(opts, kr)
}

// SetSecret stores a named env secret in the platform keyring.
// The name MUST appear under secrets: in the live config (same rule as polypus serve).
func SetSecret(envName, value string) error {
	const op = "config.SetSecret"
	envName = strings.TrimSpace(envName)
	if envName == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "secret env name is empty")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "secret value is empty")
	}
	if err := RequireDeclaredSecret(envName); err != nil {
		return sirerr.Wrap(err, sirerr.CodeInvalid, op, "secret not declared in config")
	}
	if err := operatorconfig.DefaultKeyring().Set(AppName, envName, operatorconfig.SanitizeSecret(value)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "keyring set").With("name", envName)
	}
	return nil
}

// DeleteSecret removes a keyring secret (no declaration check; idempotent when missing).
func DeleteSecret(envName string) error {
	const op = "config.DeleteSecret"
	envName = strings.TrimSpace(envName)
	if envName == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "secret env name is empty")
	}
	if err := operatorconfig.DefaultKeyring().Delete(AppName, envName); err != nil {
		if errors.Is(err, operatorconfig.ErrNotFound) {
			return nil
		}
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "keyring delete").With("name", envName)
	}
	return nil
}

// RequireDeclaredSecret reports whether envName is listed under secrets: in the live config.
func RequireDeclaredSecret(envName string) error {
	const op = "config.RequireDeclaredSecret"
	names, err := declaredSecretEnvs()
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeInvalid, op, "read declared secrets")
	}
	want := strings.TrimSpace(envName)
	for _, n := range names {
		if n == want {
			return nil
		}
	}
	if len(names) == 0 {
		return sirerr.New(sirerr.CodeInvalid, op, "secret not declared: add it under secrets: in config (run should-i-read init)").With("name", want)
	}
	return sirerr.New(sirerr.CodeInvalid, op, "secret not declared under secrets:").With("name", want)
}

func declaredSecretEnvs() ([]string, error) {
	path, err := ResolvePath("")
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, sirerr.New(sirerr.CodeNotFound, "config.declaredSecretEnvs", "no config found; run should-i-read init")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, "config.declaredSecretEnvs", "read config").With("path", path)
	}
	var file struct {
		Secrets []operatorconfig.Secret `yaml:"secrets"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&file); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeInvalid, "config.declaredSecretEnvs", "parse secrets list").With("path", path)
	}
	out := make([]string, 0, len(file.Secrets))
	for _, s := range file.Secrets {
		n := strings.TrimSpace(s.Env)
		if n == "" {
			return nil, sirerr.New(sirerr.CodeInvalid, "config.declaredSecretEnvs", "secret env name required in secrets list").With("path", path)
		}
		out = append(out, n)
	}
	return out, nil
}
