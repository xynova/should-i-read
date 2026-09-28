package secret

import (
	"errors"
	"os"
	"strings"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const ServiceName = "should-i-read"

// ErrNotFound means the secret is not in env, keyring, or SOPS file.
var ErrNotFound = errors.New("secret not found")

// Resolve returns env first, then platform keyring, then optional SOPS secrets file.
func Resolve(name string) (string, error) {
	const op = "secret.Resolve"
	name = strings.TrimSpace(name)
	if name == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "secret name is empty")
	}
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v, nil
	}
	opts := operatorconfig.Options{
		App: ServiceName,
		Secrets: []operatorconfig.Secret{
			{Env: name},
		},
	}
	if err := operatorconfig.ResolveSecrets(opts, nil); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "resolve secret").With("name", name)
	}
	v := strings.TrimSpace(os.Getenv(name))
	if v != "" {
		return v, nil
	}
	return "", sirerr.Wrap(ErrNotFound, sirerr.CodeNotFound, op, "secret unset").With("name", name)
}

// Set stores a secret in the platform keyring (service should-i-read).
func Set(name, value string) error {
	const op = "secret.Set"
	name = strings.TrimSpace(name)
	if name == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "secret name is empty")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "secret value is empty")
	}
	if err := operatorconfig.DefaultKeyring().Set(ServiceName, name, value); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "keyring set").With("name", name)
	}
	return nil
}

// Delete removes a keyring secret.
func Delete(name string) error {
	const op = "secret.Delete"
	name = strings.TrimSpace(name)
	if name == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "secret name is empty")
	}
	if err := operatorconfig.DefaultKeyring().Delete(ServiceName, name); err != nil {
		if errors.Is(err, operatorconfig.ErrNotFound) {
			return nil
		}
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "keyring delete").With("name", name)
	}
	return nil
}
