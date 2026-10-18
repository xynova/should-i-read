package config

import (
	"net/url"
	"os"
	"strings"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
	"gopkg.in/yaml.v3"

	"github.com/xynova/should-i-read/internal/secret"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	envPolypusOTLP             = "POLYPUS_OTLP_ENDPOINT"
	envOTELTracesEndpoint      = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	envOTELEndpoint            = "OTEL_EXPORTER_OTLP_ENDPOINT"
	polypusOTLPPort            = "4317"
	defaultPolypusOTLPLoopback = "127.0.0.1:" + polypusOTLPPort
	polypusOTLPPlaceholder     = "${POLYPUS_OTLP_ENDPOINT}"
)

// DerivePolypusOTLPEndpoint returns Phoenix OTLP gRPC host:port from a Polypus gateway base URL.
// Loopback gateways map to 127.0.0.1:4317. Empty or unparsable input returns "".
func DerivePolypusOTLPEndpoint(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return ""
	}
	raw := baseURL
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return ""
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return defaultPolypusOTLPLoopback
	}
	return host + ":" + polypusOTLPPort
}

// ResolvePolypusOTLPEndpoint picks the OTLP gRPC target.
// explicitOTEL and explicitOTLP are process env values snapshotted before EnvDefaults.
// yamlOTLP is the expanded polypus.otlp_endpoint (empty if missing or unresolved placeholder).
func ResolvePolypusOTLPEndpoint(explicitOTEL, explicitOTLP, yamlOTLP, derivedOTLP, baseURL string) string {
	if v := strings.TrimSpace(explicitOTEL); v != "" {
		return v
	}
	if v := strings.TrimSpace(explicitOTLP); v != "" {
		return v
	}
	if v := strings.TrimSpace(yamlOTLP); v != "" {
		return v
	}
	if v := strings.TrimSpace(derivedOTLP); v != "" {
		return v
	}
	if v := DerivePolypusOTLPEndpoint(baseURL); v != "" {
		return v
	}
	return defaultPolypusOTLPLoopback
}

// ResolveTelemetryOTLPEndpoint snapshots explicit env, applies EnvDefaults, peels YAML, then picks the exporter target.
func ResolveTelemetryOTLPEndpoint(flagPath string) (string, error) {
	const op = "config.ResolveTelemetryOTLPEndpoint"
	explicitOTEL := firstNonEmptyEnv(envOTELTracesEndpoint, envOTELEndpoint)
	explicitOTLP := strings.TrimSpace(os.Getenv(envPolypusOTLP))
	if err := operatorconfig.ApplyEnvDefaults(Options(flagPath)); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "apply env defaults")
	}
	yamlBase, yamlOTLP := peelPolypusYAML(flagPath)
	derived := strings.TrimSpace(os.Getenv(envPolypusOTLP))
	baseURL := yamlBase
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envPolypusBaseURL))
	}
	return ResolvePolypusOTLPEndpoint(explicitOTEL, explicitOTLP, yamlOTLP, derived, baseURL), nil
}

// OTLPUsesPlaintextGRPC is true for host:port or http:// endpoints (Phoenix 4317).
func OTLPUsesPlaintextGRPC(endpoint string) bool {
	ep := strings.TrimSpace(strings.ToLower(endpoint))
	if ep == "" {
		return false
	}
	if strings.HasPrefix(ep, "https://") {
		return false
	}
	return true
}

func peelPolypusYAML(flagPath string) (baseURL, otlpEndpoint string) {
	path, err := ResolvePath(flagPath)
	if err != nil || strings.TrimSpace(path) == "" {
		return "", ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var file File
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return "", ""
	}
	resolve := func(name string) (string, error) {
		return secret.Resolve(name)
	}
	baseURL, err = ExpandString(file.Polypus.BaseURL, resolve, false)
	if err != nil {
		return "", ""
	}
	otlpEndpoint, err = ExpandString(file.Polypus.OTLPEndpoint, resolve, false)
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(baseURL), strings.TrimSpace(otlpEndpoint)
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func resolvePolypusOTLPForLoad(expanded, baseURL string) string {
	if v := strings.TrimSpace(expanded); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envPolypusOTLP)); v != "" {
		return v
	}
	if v := DerivePolypusOTLPEndpoint(baseURL); v != "" {
		return v
	}
	return defaultPolypusOTLPLoopback
}
