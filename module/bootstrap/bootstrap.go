package bootstrap

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/peralta/go-observability-kit/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Runtime is the initialized observability runtime shared by module adapters.
type Runtime struct {
	Config            config.Config
	TelemetryDegraded bool
	TelemetryError    error

	tracerProvider *sdktrace.TracerProvider
	shutdownFns    []func(context.Context) error
}

// Config aliases the environment contract type.
type Config = config.Config

// Init validates configuration and initializes observability providers.
func Init(ctx context.Context, cfg Config) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("bootstrap init: %w", err)
	}

	rt := &Runtime{Config: cfg}

	tp, err := initTracing(ctx, cfg)
	if err != nil {
		rt.MarkTelemetryDegraded(fmt.Errorf("tracing init degraded: %w", err))
	}
	if tp == nil {
		tp = sdktrace.NewTracerProvider()
	}
	rt.tracerProvider = tp
	rt.shutdownFns = append(rt.shutdownFns, func(sctx context.Context) error {
		return tp.Shutdown(sctx)
	})

	otel.SetTracerProvider(tp)
	return rt, nil
}

// Shutdown shuts down providers and returns combined error if any.
func Shutdown(ctx context.Context, rt *Runtime) error {
	if rt == nil {
		return nil
	}

	var errs []string
	for i := len(rt.shutdownFns) - 1; i >= 0; i-- {
		if err := rt.shutdownFns[i](ctx); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("bootstrap shutdown: %s", strings.Join(errs, "; "))
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

func initTracing(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	sampler, err := buildSampler(cfg.OTELTracesSampler, cfg.OTELTracesSamplerArg)
	if err != nil {
		return nil, fmt.Errorf("invalid sampler config: %w", err)
	}

	resourceAttrs, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.DeploymentEnv),
		),
	)

	endpoint := normalizeOTLPEndpoint(cfg.OTLPExporterEndpoint)
	if endpoint == "" {
		return sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sampler),
			sdktrace.WithResource(resourceAttrs),
		), nil
	}

	protocol := strings.ToLower(strings.TrimSpace(cfg.OTLPExporterProtocol))
	if protocol == "" {
		protocol = "grpc"
	}
	if protocol != "grpc" {
		return nil, fmt.Errorf("unsupported OTLP protocol %q; supported protocol: grpc", protocol)
	}

	exporterCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	exporterOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
	if shouldUseInsecureTransport(cfg.OTLPExporterEndpoint) {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(exporterCtx, exporterOpts...)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resourceAttrs),
		sdktrace.WithSampler(sampler),
	)

	return tp, nil
}

func normalizeOTLPEndpoint(raw string) string {
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func shouldUseInsecureTransport(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" {
		return true
	}
	return strings.EqualFold(u.Scheme, "http")
}

func buildSampler(name, arg string) (sdktrace.Sampler, error) {
	samplerName := strings.ToLower(strings.TrimSpace(name))
	if samplerName == "" {
		samplerName = "parentbased_traceidratio"
	}

	ratio := 1.0
	if strings.TrimSpace(arg) != "" {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(arg), 64)
		if err != nil {
			return nil, fmt.Errorf("parse sampler arg %q: %w", arg, err)
		}
		if parsed < 0 || parsed > 1 {
			return nil, fmt.Errorf("sampler arg out of range [0,1]: %v", parsed)
		}
		ratio = parsed
	}

	switch samplerName {
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratio), nil
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio)), nil
	default:
		return nil, fmt.Errorf("unsupported sampler %q", samplerName)
	}
}
