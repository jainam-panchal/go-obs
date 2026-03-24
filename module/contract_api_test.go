package module_test

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jainam-panchal/go-obs/module/bootstrap"
	"github.com/jainam-panchal/go-obs/module/dbx"
	"github.com/jainam-panchal/go-obs/module/ginx"
	"github.com/jainam-panchal/go-obs/module/health"
	"github.com/jainam-panchal/go-obs/module/metrics"
	"github.com/jainam-panchal/go-obs/module/workerx"
	"gorm.io/gorm"
)

func TestPublicAPISignaturesCompile(t *testing.T) {
	var (
		_ func(context.Context, bootstrap.Config) (*bootstrap.Runtime, error) = bootstrap.Init
		_ func(context.Context, *bootstrap.Runtime) error                     = bootstrap.Shutdown
		_ func(*bootstrap.Runtime) gin.HandlerFunc                            = ginx.Middleware
		_ func(*bootstrap.Runtime) asynq.MiddlewareFunc                       = workerx.AsynqMiddleware
		_ func(*gorm.DB, *bootstrap.Runtime, ...dbx.Option) *gorm.DB          = dbx.WrapGORM
		_ func(gin.IRouter, ...health.Check)                                  = health.RegisterRoutes
		_ func(gin.IRouter)                                                   = metrics.RegisterRoute
	)
}
