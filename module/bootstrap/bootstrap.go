package bootstrap

import (
	"context"
	"fmt"

	"github.com/peralta/go-observability-kit/config"
)

// Runtime is the initialized observability runtime shared by module adapters.
type Runtime struct {
	Config            config.Config
	TelemetryDegraded bool
	TelemetryError    error
}

// Config aliases the environment contract type.
type Config = config.Config

// Init validates configuration and initializes a runtime skeleton.
func Init(_ context.Context, cfg Config) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("bootstrap init: %w", err)
	}

	return &Runtime{Config: cfg}, nil
}

// Shutdown provides a no-op skeleton that can be extended with providers.
func Shutdown(_ context.Context, rt *Runtime) error {
	if rt == nil {
		return nil
	}
	return nil
}

// MarkTelemetryDegraded records non-fatal telemetry initialization errors.
func (rt *Runtime) MarkTelemetryDegraded(err error) {
	if rt == nil || err == nil {
		return
	}
	rt.TelemetryDegraded = true
	rt.TelemetryError = err
}
