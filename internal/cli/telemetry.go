package cli

import (
	"context"

	ollicli "github.com/behaviorengineering/olly/pkg/cli"
)

var otelLife *ollicli.Lifecycle

func startTelemetry(serviceName string) error {
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
	return err
}

func stopTelemetry() error {
	if otelLife == nil {
		return nil
	}
	return otelLife.Stop(context.Background())
}
