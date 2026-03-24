package ginx

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/peralta/go-observability-kit/config"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestMiddlewareSetsRequestIDAndPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(&bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}}))
	r.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if rid := w.Header().Get("X-Request-Id"); rid == "" {
		t.Fatal("expected X-Request-Id header")
	}
	if tid := w.Header().Get("X-Trace-Id"); tid == "" {
		t.Fatal("expected X-Trace-Id header")
	}
}

func TestMiddlewareContinuesW3CTraceContext(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(prev)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(&bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}}))
	r.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	traceID := "1234567890abcdef1234567890abcdef"
	parentSpanID := "1122334455667788"
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("traceparent", "00-"+traceID+"-"+parentSpanID+"-01")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Trace-Id"); got != traceID {
		t.Fatalf("trace propagation mismatch: got=%q want=%q", got, traceID)
	}
}

func TestMiddlewareRouteCardinalityStabilityUnderLoad(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rt := &bootstrap.Runtime{Config: config.Config{ServiceName: "svc-card", DeploymentEnv: "bench"}}
	r := gin.New()
	r.Use(Middleware(rt))

	for i := 0; i < 500; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/"+strconv.Itoa(i), nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("unexpected status=%d", w.Code)
		}
	}

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	seriesCount := 0
	for _, mf := range mfs {
		if mf.GetName() != "http_server_requests_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range metric.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["service"] == "svc-card" && labels["env"] == "bench" && labels["method"] == "GET" && labels["status_class"] == "4xx" {
				if labels["route"] != "_unmatched" {
					t.Fatalf("unexpected route label: %s", labels["route"])
				}
				seriesCount++
			}
		}
	}
	if seriesCount != 1 {
		t.Fatalf("expected one stable series for unmatched routes, got=%d", seriesCount)
	}
}

func TestMiddlewareUsesLowCardinalityFallbackRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(&bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/123", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "/users/123") {
		t.Fatal("unexpected raw path leak in fallback route")
	}
}
