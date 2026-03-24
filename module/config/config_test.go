package config

import "testing"

func TestLoadFromEnvDefaultsAndRequired(t *testing.T) {
	t.Setenv("SERVICE_NAME", "svc-a")
	t.Setenv("SERVICE_VERSION", "1.2.3")
	t.Setenv("DEPLOYMENT_ENV", "dev")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")
	t.Setenv("OTEL_TRACES_SAMPLER", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("METRICS_ENABLED", "")
	t.Setenv("PPROF_ENABLED", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.OTLPExporterProtocol != "grpc" {
		t.Fatalf("OTLPExporterProtocol = %q, want %q", cfg.OTLPExporterProtocol, "grpc")
	}
	if cfg.OTELTracesSampler != "parentbased_traceidratio" {
		t.Fatalf("OTELTracesSampler = %q, want %q", cfg.OTELTracesSampler, "parentbased_traceidratio")
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if !cfg.MetricsEnabled {
		t.Fatalf("MetricsEnabled = false, want true")
	}
	if cfg.PPROFEnabled {
		t.Fatalf("PPROFEnabled = true, want false")
	}
}

func TestLoadFromEnvMissingRequired(t *testing.T) {
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("DEPLOYMENT_ENV", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for missing required env")
	}
}
