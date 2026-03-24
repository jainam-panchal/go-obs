package bootstrap

import (
	"context"
	"testing"

	"github.com/peralta/go-observability-kit/config"
)

func TestInitValidatesConfig(t *testing.T) {
	cfg := config.Config{}
	_, err := Init(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected init validation error")
	}
}

func TestInitAndShutdown(t *testing.T) {
	cfg := config.Config{
		ServiceName:          "svc-a",
		ServiceVersion:       "1.0.0",
		DeploymentEnv:        "dev",
		OTLPExporterEndpoint: "http://otel-collector:4317",
	}

	rt, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if err := Shutdown(context.Background(), rt); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
