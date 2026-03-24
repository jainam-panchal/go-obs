package health

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const defaultReadinessTimeout = 2 * time.Second

var errLegacyCheckRunning = errors.New("legacy readiness check still running; migrate to FnCtx for cancellation")

// Check represents a readiness dependency probe.
type Check struct {
	Name string
	// Fn is a legacy readiness probe signature. Prefer FnCtx for timeout/cancellation support.
	Fn func() error
	// FnCtx is the recommended readiness probe signature with context support.
	FnCtx func(context.Context) error
}

// RegisterRoutes registers liveness and readiness routes.
func RegisterRoutes(router gin.IRouter, checks ...Check) {
	legacyChecks := make([]*legacyCheckRunner, len(checks))
	for i, chk := range checks {
		if chk.Fn != nil {
			legacyChecks[i] = &legacyCheckRunner{fn: chk.Fn}
		}
	}

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/readyz", func(c *gin.Context) {
		for i, chk := range checks {
			if chk.Fn == nil && chk.FnCtx == nil {
				continue
			}

			if chk.FnCtx != nil {
				checkCtx, cancel := context.WithTimeout(c.Request.Context(), defaultReadinessTimeout)
				err := chk.FnCtx(checkCtx)
				cancel()
				if err != nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "check": chk.Name, "error": err.Error()})
					return
				}
				continue
			}

			// Legacy checks have no cancellation. We cap request wait time and keep only one
			// concurrent execution per check to avoid per-request goroutine accumulation.
			if err := legacyChecks[i].Run(defaultReadinessTimeout); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "check": chk.Name, "error": err.Error()})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}

type legacyCheckRunner struct {
	fn func() error

	mu      sync.Mutex
	running bool
	done    chan error
}

func (r *legacyCheckRunner) Run(timeout time.Duration) error {
	r.mu.Lock()
	if r.running {
		done := r.done
		r.mu.Unlock()
		select {
		case err := <-done:
			r.mu.Lock()
			r.running = false
			r.done = nil
			r.mu.Unlock()
			return err
		default:
			return errLegacyCheckRunning
		}
	}
	r.running = true
	r.done = make(chan error, 1)
	done := r.done
	r.mu.Unlock()

	go func() {
		done <- r.fn()
	}()

	select {
	case err := <-done:
		r.mu.Lock()
		r.running = false
		r.done = nil
		r.mu.Unlock()
		return err
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}
