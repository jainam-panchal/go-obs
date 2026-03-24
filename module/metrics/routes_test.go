package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRouteServesMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoute(r)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/metrics status=%d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "go_gc_duration_seconds") {
		t.Fatal("expected prometheus payload")
	}
}
