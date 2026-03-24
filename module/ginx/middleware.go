package ginx

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	httpMetricsOnce sync.Once

	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpInflightRequests *prometheus.GaugeVec
)

// Middleware instruments inbound HTTP requests with request IDs, metrics, traces, and logs.
func Middleware(rt *bootstrap.Runtime) gin.HandlerFunc {
	initHTTPMetrics()

	service := "unknown-service"
	env := "unknown-env"
	if rt != nil {
		if rt.Config.ServiceName != "" {
			service = rt.Config.ServiceName
		}
		if rt.Config.DeploymentEnv != "" {
			env = rt.Config.DeploymentEnv
		}
	}

	tracer := otel.Tracer("go-observability-kit/ginx")

	return func(c *gin.Context) {
		start := time.Now()
		httpInflightRequests.WithLabelValues(service, env).Inc()
		defer httpInflightRequests.WithLabelValues(service, env).Dec()

		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Writer.Header().Set("X-Request-Id", requestID)

		traceID := c.GetHeader("X-Trace-Id")
		if traceID == "" {
			if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
				traceID = sc.TraceID().String()
			} else {
				traceID = strings.ReplaceAll(uuid.NewString(), "-", "")
			}
		}

		spanName := c.FullPath()
		if spanName == "" {
			spanName = c.Request.URL.Path
		}
		ctx, span := tracer.Start(c.Request.Context(), spanName)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "_unmatched"
		}
		status := c.Writer.Status()
		statusClass := fmt.Sprintf("%dxx", status/100)

		httpRequestsTotal.WithLabelValues(service, env, c.Request.Method, route, statusClass).Inc()
		httpRequestDuration.WithLabelValues(service, env, c.Request.Method, route, statusClass).Observe(time.Since(start).Seconds())

		if len(c.Errors) > 0 || status >= 500 {
			span.SetStatus(codes.Error, c.Errors.String())
		}
		span.End()

		spanID := ""
		if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
			spanID = sc.SpanID().String()
		}
		if spanID == "" && len(traceID) >= 16 {
			spanID = traceID[:16]
		}

		logJSON(map[string]any{
			"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
			"level":       "info",
			"msg":         "http_request",
			"service":     service,
			"env":         env,
			"trace_id":    traceID,
			"span_id":     spanID,
			"request_id":  requestID,
			"route":       route,
			"status_code": status,
			"latency_ms":  time.Since(start).Milliseconds(),
		})
	}
}

func initHTTPMetrics() {
	httpMetricsOnce.Do(func() {
		httpRequestsTotal = mustRegisterCounterVec("http_server_requests_total", "Total HTTP requests", []string{"service", "env", "method", "route", "status_class"})
		httpRequestDuration = mustRegisterHistogramVec("http_server_request_duration_seconds", "HTTP request duration", []string{"service", "env", "method", "route", "status_class"})
		httpInflightRequests = mustRegisterGaugeVec("http_server_inflight_requests", "In-flight HTTP requests", []string{"service", "env"})
	})
}

func mustRegisterCounterVec(name, help string, labels []string) *prometheus.CounterVec {
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: help}, labels)
	if err := prometheus.Register(cv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing
			}
		}
	}
	return cv
}

func mustRegisterHistogramVec(name, help string, labels []string) *prometheus.HistogramVec {
	hv := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: name, Help: help, Buckets: prometheus.DefBuckets}, labels)
	if err := prometheus.Register(hv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
				return existing
			}
		}
	}
	return hv
}

func mustRegisterGaugeVec(name, help string, labels []string) *prometheus.GaugeVec {
	gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, labels)
	if err := prometheus.Register(gv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.GaugeVec); ok {
				return existing
			}
		}
	}
	return gv
}

func logJSON(fields map[string]any) {
	b, _ := json.Marshal(fields)
	log.Println(string(b))
}
