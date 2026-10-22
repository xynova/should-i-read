package cli

import (
	"context"

	ollicli "github.com/behaviorengineering/olly/pkg/cli"
	"github.com/behaviorengineering/olly/pkg/olly"
	"go.opentelemetry.io/otel"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

var otelLife *ollicli.Lifecycle

func startTelemetry(serviceName, configFlagPath string) error {
	// Export failures otherwise spam stderr and corrupt TUI redraws.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {}))
	endpoint, err := telemetryOTLPEndpoint(configFlagPath)
	if err != nil {
		return err
	}
	cfg := ollicli.ConfigFromEnv(serviceName)
	cfg.Dump.Dir = "logs/failures"
	if cfg.Dump.MaxAgeHours == 0 {
		cfg.Dump.MaxAgeHours = 48
	}
	if cfg.Dump.MaxFiles == 0 {
		cfg.Dump.MaxFiles = 20
	}
	cfg.OTLPEndpoint = endpoint
	if config.OTLPUsesPlaintextGRPC(endpoint) {
		cfg.Insecure = olly.BoolPtr(true)
	}
	otelLife, err = ollicli.Start(cfg)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, "cli.startTelemetry", "olly start")
	}
	return nil
}

func telemetryOTLPEndpoint(flagPath string) (string, error) {
	const op = "cli.telemetryOTLPEndpoint"
	endpoint, err := config.ResolveTelemetryOTLPEndpoint(flagPath)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "resolve Phoenix OTLP endpoint")
	}
	return endpoint, nil
}

func stopTelemetry() error {
	if otelLife == nil {
		return nil
	}
	return otelLife.Stop(context.Background())
}
