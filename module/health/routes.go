package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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
			if chk.Fn != nil {
				if err := chk.Fn(); err != nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "check": chk.Name, "error": err.Error()})
					return
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
