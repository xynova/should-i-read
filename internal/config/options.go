package config

import (
	"os"
	"strings"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"

	"github.com/xynova/should-i-read/internal/oauthcred"
)

// Options builds operatorconfig.Options for this host (no Secrets until load time).
func Options(flagPath string) operatorconfig.Options {
	return operatorconfig.Options{
		App:            AppName,
		ConfigEnv:      envConfigPath,
		ConfigFlagPath: strings.TrimSpace(flagPath),
		Filename:       "config.yaml",
		EnvDefaults: []operatorconfig.EnvDefault{
			{Env: envPolypusBaseURL, Value: defaultPolypusBaseURL},
			{Env: envPolypusOTLP, Derive: func() (string, error) {
				return DerivePolypusOTLPEndpoint(os.Getenv(envPolypusBaseURL)), nil
			}},
		},
	}
}

// DefaultSecrets is the init template secrets: list (OAuth client env names only).
func DefaultSecrets() []operatorconfig.Secret {
	return []operatorconfig.Secret{
		{Env: envPolypusBaseURL},
		{Env: oauthcred.EnvGmailClientID},
		{Env: oauthcred.EnvGmailClientSecret},
		{Env: oauthcred.EnvOutlookClientID},
	}
}
