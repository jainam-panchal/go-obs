package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
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

func TestRegisterRoutesParallelProbesSlowFnCtx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	var inFlight atomic.Int64
	var maxInFlight atomic.Int64

	RegisterRoutes(r, Check{
		Name: "slow",
		FnCtx: func(ctx context.Context) error {
			cur := inFlight.Add(1)
			for {
				prev := maxInFlight.Load()
				if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
					break
				}
			}
			defer inFlight.Add(-1)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(150 * time.Millisecond):
				return nil
			}
		},
	})

	var wg sync.WaitGroup
	errCh := make(chan int, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			errCh <- w.Code
		}()
	}
	wg.Wait()
	close(errCh)

	for code := range errCh {
		if code != http.StatusOK {
			t.Fatalf("unexpected status from parallel probe: %d", code)
		}
	}
	if maxInFlight.Load() < 2 {
		t.Fatalf("expected concurrent readiness probes, max_inflight=%d", maxInFlight.Load())
	}
}

func TestRegisterRoutesParallelProbesOneSlowOneFastDependency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	RegisterRoutes(
		r,
		Check{Name: "fast", FnCtx: func(ctx context.Context) error { return nil }},
		Check{Name: "slow", FnCtx: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return nil
			}
		}},
	)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if w.Code != http.StatusOK {
				t.Errorf("status=%d", w.Code)
			}
		}()
	}
	wg.Wait()
}
