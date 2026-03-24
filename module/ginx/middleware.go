package ginx

import (
	"github.com/gin-gonic/gin"
	"github.com/peralta/go-observability-kit/bootstrap"
)

// Middleware is a pass-through skeleton for HTTP instrumentation.
func Middleware(_ *bootstrap.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
