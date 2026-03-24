package bootstrap

import (
	"context"
	"testing"

	"github.com/jainam-panchal/go-obs/module/config"
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

func TestInitSupportsHTTPProtobufProtocol(t *testing.T) {
	cfg := config.Config{
		ServiceName:          "svc-a",
		ServiceVersion:       "1.0.0",
		DeploymentEnv:        "dev",
		OTLPExporterEndpoint: "http://otel-collector:4318",
		OTLPExporterProtocol: "http/protobuf",
	}

	rt, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if rt.TelemetryDegraded {
		t.Fatalf("expected non-degraded runtime for http/protobuf, got: %v", rt.TelemetryError)
	}
}

func TestBuildSamplerConfig(t *testing.T) {
	cases := []struct {
		name      string
		sampler   string
		arg       string
		wantError bool
	}{
		{name: "default", sampler: "", arg: "", wantError: false},
		{name: "always_on", sampler: "always_on", arg: "", wantError: false},
		{name: "ratio", sampler: "traceidratio", arg: "0.2", wantError: false},
		{name: "invalid_ratio", sampler: "traceidratio", arg: "2.0", wantError: true},
		{name: "invalid_sampler", sampler: "foo", arg: "", wantError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildSampler(tc.sampler, tc.arg)
			if tc.wantError && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
