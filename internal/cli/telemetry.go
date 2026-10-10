package cli

import (
	"context"

	ollicli "github.com/behaviorengineering/olly/pkg/cli"
	"go.opentelemetry.io/otel"

	"github.com/xynova/should-i-read/internal/sirerr"
)

var otelLife *ollicli.Lifecycle

func startTelemetry(serviceName string) error {
	// Export failures (no collector on :4319) otherwise spam stderr and corrupt TUI redraws.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {}))
	cfg := ollicli.ConfigFromEnv(serviceName)
	cfg.Dump.Dir = "logs/failures"
	if cfg.Dump.MaxAgeHours == 0 {
		cfg.Dump.MaxAgeHours = 48
	}
	if cfg.Dump.MaxFiles == 0 {
		cfg.Dump.MaxFiles = 20
	}
	var err error
	otelLife, err = ollicli.Start(cfg)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, "cli.startTelemetry", "olly start")
	}
	return nil
}

func stopTelemetry() error {
	if otelLife == nil {
		return nil
	}
	return otelLife.Stop(context.Background())
}
