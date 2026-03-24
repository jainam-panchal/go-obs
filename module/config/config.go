package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultOTLPProtocol = "grpc"
	defaultSampler      = "parentbased_traceidratio"
	defaultLogLevel     = "info"
)

var (
	ErrMissingServiceName    = errors.New("missing required env SERVICE_NAME")
	ErrMissingServiceVersion = errors.New("missing required env SERVICE_VERSION")
	ErrMissingDeploymentEnv  = errors.New("missing required env DEPLOYMENT_ENV")
	ErrMissingOTLPEndpoint   = errors.New("missing required env OTEL_EXPORTER_OTLP_ENDPOINT")
	ErrInvalidMetricsEnabled = errors.New("invalid METRICS_ENABLED")
	ErrInvalidPPROFEnabled   = errors.New("invalid PPROF_ENABLED")
)

// Config is the required environment contract for the module.
type Config struct {
	ServiceName          string
	ServiceVersion       string
	DeploymentEnv        string
	OTLPExporterEndpoint string
	OTLPExporterProtocol string
	OTELTracesSampler    string
	OTELTracesSamplerArg string
	LogLevel             string
	MetricsEnabled       bool
	PPROFEnabled         bool
}

// LoadFromEnv loads config from environment and applies defaults.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		ServiceName:          os.Getenv("SERVICE_NAME"),
		ServiceVersion:       os.Getenv("SERVICE_VERSION"),
		DeploymentEnv:        os.Getenv("DEPLOYMENT_ENV"),
		OTLPExporterEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTLPExporterProtocol: getenvOrDefault("OTEL_EXPORTER_OTLP_PROTOCOL", defaultOTLPProtocol),
		OTELTracesSampler:    getenvOrDefault("OTEL_TRACES_SAMPLER", defaultSampler),
		OTELTracesSamplerArg: os.Getenv("OTEL_TRACES_SAMPLER_ARG"),
		LogLevel:             getenvOrDefault("LOG_LEVEL", defaultLogLevel),
		MetricsEnabled:       true,
		PPROFEnabled:         false,
	}

	if metricsEnabled := os.Getenv("METRICS_ENABLED"); metricsEnabled != "" {
		v, err := strconv.ParseBool(metricsEnabled)
		if err != nil {
			return Config{}, fmt.Errorf("%w: %v", ErrInvalidMetricsEnabled, err)
		}
		cfg.MetricsEnabled = v
	}

	if pprofEnabled := os.Getenv("PPROF_ENABLED"); pprofEnabled != "" {
		v, err := strconv.ParseBool(pprofEnabled)
		if err != nil {
			return Config{}, fmt.Errorf("%w: %v", ErrInvalidPPROFEnabled, err)
		}
		cfg.PPROFEnabled = v
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate ensures required values are present.
func (c Config) Validate() error {
	switch {
	case c.ServiceName == "":
		return ErrMissingServiceName
	case c.ServiceVersion == "":
		return ErrMissingServiceVersion
	case c.DeploymentEnv == "":
		return ErrMissingDeploymentEnv
	case c.OTLPExporterEndpoint == "":
		return ErrMissingOTLPEndpoint
	default:
		return nil
	}
}

func getenvOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
