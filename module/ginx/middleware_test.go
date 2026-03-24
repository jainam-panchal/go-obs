package ginx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/peralta/go-observability-kit/config"
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
