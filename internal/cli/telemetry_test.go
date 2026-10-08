package cli

import (
	"testing"

	"github.com/spf13/cobra"
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
