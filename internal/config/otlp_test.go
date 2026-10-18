package config

import (
	"os"
	"path/filepath"
	"testing"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
)

func TestDerivePolypusOTLPEndpoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"https://polypus.manticore-powan.ts.net", "polypus.manticore-powan.ts.net:4317"},
		{"https://polypus.manticore-powan.ts.net:443/", "polypus.manticore-powan.ts.net:4317"},
		{"https://polypus.manticore-powan.ts.net/v1", "polypus.manticore-powan.ts.net:4317"},
		{"http://127.0.0.1:1320", "127.0.0.1:4317"},
		{"http://localhost:1320", "127.0.0.1:4317"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		got := DerivePolypusOTLPEndpoint(tc.in)
		if got != tc.want {
			t.Errorf("DerivePolypusOTLPEndpoint(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolvePolypusOTLPEndpoint_precedence(t *testing.T) {
	t.Parallel()
	base := "https://polypus.manticore-powan.ts.net"
	derived := DerivePolypusOTLPEndpoint(base)
	if derived != "polypus.manticore-powan.ts.net:4317" {
		t.Fatalf("derived %q", derived)
	}
	if got := ResolvePolypusOTLPEndpoint("collector:4317", "ignored:1", "yaml:1", derived, base); got != "collector:4317" {
		t.Fatalf("explicit OTEL: %q", got)
	}
	if got := ResolvePolypusOTLPEndpoint("", "explicit-otlp:4317", "yaml:1", derived, base); got != "explicit-otlp:4317" {
		t.Fatalf("explicit POLYPUS_OTLP: %q", got)
	}
	if got := ResolvePolypusOTLPEndpoint("", "", "yaml-host:4317", derived, base); got != "yaml-host:4317" {
		t.Fatalf("yaml: %q", got)
	}
	if got := ResolvePolypusOTLPEndpoint("", "", "", derived, base); got != derived {
		t.Fatalf("derived env: %q", got)
	}
	if got := ResolvePolypusOTLPEndpoint("", "", "", "", base); got != derived {
		t.Fatalf("derive from base: %q", got)
	}
	if got := ResolvePolypusOTLPEndpoint("", "", "", "", ""); got != defaultPolypusOTLPLoopback {
		t.Fatalf("empty fallback: %q", got)
	}
}

func TestOTLPUsesPlaintextGRPC(t *testing.T) {
	t.Parallel()
	if !OTLPUsesPlaintextGRPC("polypus.manticore-powan.ts.net:4317") {
		t.Fatal("host:port should be plaintext")
	}
	if !OTLPUsesPlaintextGRPC("http://127.0.0.1:4317") {
		t.Fatal("http should be plaintext")
	}
	if OTLPUsesPlaintextGRPC("https://collector.example:4317") {
		t.Fatal("https should use TLS")
	}
}

func TestOptionsDerivesPolypusOTLP(t *testing.T) {
	t.Setenv(envPolypusBaseURL, "https://polypus.manticore-powan.ts.net")
	t.Setenv(envPolypusOTLP, "")
	if err := operatorconfig.ApplyEnvDefaults(Options("")); err != nil {
		t.Fatal(err)
	}
	got := os.Getenv(envPolypusOTLP)
	if got != "polypus.manticore-powan.ts.net:4317" {
		t.Fatalf("derived env %q", got)
	}
}

func TestResolveTelemetryOTLPEndpoint(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("polypus:\n  base_url: \"${POLYPUS_BASE_URL}\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envOTELEndpoint, "")
	t.Setenv(envOTELTracesEndpoint, "")
	t.Setenv(envPolypusOTLP, "")
	t.Setenv(envPolypusBaseURL, "https://polypus.manticore-powan.ts.net")
	got, err := ResolveTelemetryOTLPEndpoint(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "polypus.manticore-powan.ts.net:4317" {
		t.Fatalf("derived: %q", got)
	}

	t.Setenv(envOTELEndpoint, "collector:4317")
	got, err = ResolveTelemetryOTLPEndpoint(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "collector:4317" {
		t.Fatalf("otel wins: %q", got)
	}
}

func TestResolveTelemetryOTLPEndpointYAMLLiteral(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	body := "polypus:\n  base_url: \"http://127.0.0.1:1320\"\n  otlp_endpoint: \"yaml-host:4317\"\n"
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envOTELEndpoint, "")
	t.Setenv(envOTELTracesEndpoint, "")
	t.Setenv(envPolypusOTLP, "")
	t.Setenv(envPolypusBaseURL, "http://127.0.0.1:1320")
	got, err := ResolveTelemetryOTLPEndpoint(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "yaml-host:4317" {
		t.Fatalf("yaml literal: %q", got)
	}
}
