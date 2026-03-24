package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterRoute registers the Prometheus scrape endpoint.
func RegisterRoute(router gin.IRouter) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
