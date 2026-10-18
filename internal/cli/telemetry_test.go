package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/config"
)

func TestSkipTelemetryForCmd_mailReadiness(t *testing.T) {
	root := &cobra.Command{Use: "should-i-read"}
	mail := &cobra.Command{Use: "mail"}
	readiness := &cobra.Command{Use: "readiness"}
	mail.AddCommand(readiness)
	root.AddCommand(mail)

	if !skipTelemetryForCmd(readiness) {
		t.Fatal("expected mail readiness to skip telemetry")
	}
}

func TestSkipTelemetryForCmd_mailReport(t *testing.T) {
	root := &cobra.Command{Use: "should-i-read"}
	mail := &cobra.Command{Use: "mail"}
	report := &cobra.Command{Use: "report"}
	mail.AddCommand(report)
	root.AddCommand(mail)

	if skipTelemetryForCmd(report) {
		t.Fatal("expected mail report to keep telemetry")
	}
}

func TestSkipTelemetryForCmd_polypus(t *testing.T) {
	root := &cobra.Command{Use: "should-i-read"}
	polypus := &cobra.Command{Use: "polypus"}
	check := &cobra.Command{Use: "check"}
	polypus.AddCommand(check)
	root.AddCommand(polypus)

	if !skipTelemetryForCmd(check) {
		t.Fatal("expected polypus check to skip telemetry")
	}
}

func TestTelemetryOTLPEndpoint_derivesPhoenix(t *testing.T) {
	cfgFile := writeTelemetryTestConfig(t, "polypus:\n  base_url: \"${POLYPUS_BASE_URL}\"\n")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("POLYPUS_OTLP_ENDPOINT", "")
	t.Setenv("POLYPUS_BASE_URL", "https://polypus.manticore-powan.ts.net")
	got, err := telemetryOTLPEndpoint(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "polypus.manticore-powan.ts.net:4317" {
		t.Fatalf("got %q", got)
	}
	if !config.OTLPUsesPlaintextGRPC(got) {
		t.Fatal("expected plaintext gRPC Insecure for host:port")
	}
}

func TestTelemetryOTLPEndpoint_otelWins(t *testing.T) {
	cfgFile := writeTelemetryTestConfig(t, "polypus:\n  base_url: \"${POLYPUS_BASE_URL}\"\n")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("POLYPUS_OTLP_ENDPOINT", "")
	t.Setenv("POLYPUS_BASE_URL", "https://polypus.manticore-powan.ts.net")
	got, err := telemetryOTLPEndpoint(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "collector:4317" {
		t.Fatalf("got %q", got)
	}
}

func writeTelemetryTestConfig(t *testing.T, body string) string {
	t.Helper()
	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgFile
}
