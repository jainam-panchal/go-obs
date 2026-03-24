package health

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const defaultReadinessTimeout = 2 * time.Second

// Check represents a readiness dependency probe.
type Check struct {
	Name string
	Fn   func() error
}

// RegisterRoutes registers liveness and readiness routes.
func RegisterRoutes(router gin.IRouter, checks ...Check) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/readyz", func(c *gin.Context) {
		for _, chk := range checks {
			if chk.Fn == nil {
				continue
			}

			errCh := make(chan error, 1)
			go func(fn func() error) {
				errCh <- fn()
			}(chk.Fn)

			select {
			case err := <-errCh:
				if err != nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "check": chk.Name, "error": err.Error()})
					return
				}
			case <-time.After(defaultReadinessTimeout):
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "check": chk.Name, "error": "readiness check timeout"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
