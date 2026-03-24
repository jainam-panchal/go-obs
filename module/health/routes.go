package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const defaultReadinessTimeout = 2 * time.Second

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
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/readyz", func(c *gin.Context) {
		for _, chk := range checks {
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

			// Legacy checks cannot be canceled; run directly to avoid goroutine leaks.
			if err := chk.Fn(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "check": chk.Name, "error": err.Error()})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
