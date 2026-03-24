package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesHealthzReadyz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, Check{Name: "ok", FnCtx: func(ctx context.Context) error { return nil }})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/healthz status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/readyz status=%d", w.Code)
	}
}

func TestRegisterRoutesReadyzFailureAndTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, Check{Name: "bad", FnCtx: func(ctx context.Context) error { return errors.New("boom") }})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz status=%d, want 503", w.Code)
	}

	r2 := gin.New()
	RegisterRoutes(r2, Check{Name: "slow", FnCtx: func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
			return nil
		}
	}})
	w = httptest.NewRecorder()
	r2.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz timeout status=%d, want 503", w.Code)
	}
}

func TestRegisterRoutesLegacyFnTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, Check{Name: "legacy-slow", Fn: func() error {
		time.Sleep(3 * time.Second)
		return nil
	}})

	start := time.Now()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz status=%d, want 503", w.Code)
	}
	if elapsed := time.Since(start); elapsed > (defaultReadinessTimeout + 500*time.Millisecond) {
		t.Fatalf("legacy check did not timeout quickly, elapsed=%s", elapsed)
	}
}

func TestRegisterRoutesLegacyFnReturnsRunningError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	started := make(chan struct{})
	release := make(chan struct{})
	r := gin.New()
	RegisterRoutes(r, Check{Name: "legacy-blocked", Fn: func() error {
		close(started)
		<-release
		return nil
	}})

	w1 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	go r.ServeHTTP(w1, req)
	<-started

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w2.Code != http.StatusServiceUnavailable {
		t.Fatalf("second /readyz status=%d, want 503", w2.Code)
	}

	close(release)
}

func TestRegisterRoutesLegacyFnUsesRecentLastResultWhileRunning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	started := make(chan struct{})
	release := make(chan struct{})
	blockNext := false

	r := gin.New()
	RegisterRoutes(r, Check{Name: "legacy-cached", Fn: func() error {
		if blockNext {
			close(started)
			<-release
		}
		return nil
	}})

	// Seed a successful last result.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("seed /readyz status=%d, want 200", w.Code)
	}

	blockNext = true
	w1 := httptest.NewRecorder()
	go r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	<-started

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("concurrent /readyz status=%d, want 200", w2.Code)
	}

	close(release)
}
