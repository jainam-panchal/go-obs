package dbx

import (
	"context"
	"sync"
	"time"

	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// Option is reserved for DB instrumentation options in later phases.
type Option func(*options)

type options struct{}

var (
	dbMetricsOnce sync.Once
	dbQueries     *prometheus.CounterVec
	dbLatency     *prometheus.HistogramVec
)

const (
	startKey = "obs_db_start"
	spanKey  = "obs_db_span"
)

// WrapGORM registers lightweight query metrics and trace callbacks.
func WrapGORM(db *gorm.DB, rt *bootstrap.Runtime, _ ...Option) *gorm.DB {
	if db == nil {
		return nil
	}
	initDBMetrics()

	service := "unknown-service"
	env := "unknown-env"
	if rt != nil {
		if rt.Config.ServiceName != "" {
			service = rt.Config.ServiceName
		}
		if rt.Config.DeploymentEnv != "" {
			env = rt.Config.DeploymentEnv
		}
	}

	before := func(name string) func(*gorm.DB) {
		return func(tx *gorm.DB) {
			tx.InstanceSet(startKey, time.Now())
			ctx := tx.Statement.Context
			if ctx == nil {
				ctx = context.Background()
			}
			ctx, span := otel.Tracer("go-observability-kit/dbx").Start(ctx, "db."+name)
			tx.Statement.Context = ctx
			tx.InstanceSet(spanKey, span)
		}
	}

	observe := func(name string) func(*gorm.DB) {
		return func(tx *gorm.DB) {
			result := "success"
			if tx.Error != nil {
				result = "error"
			}
			dbQueries.WithLabelValues(service, env, name, result).Inc()
			if v, ok := tx.InstanceGet(startKey); ok {
				if t, ok := v.(time.Time); ok {
					dbLatency.WithLabelValues(service, env, name, result).Observe(time.Since(t).Seconds())
				}
			}
			if v, ok := tx.InstanceGet(spanKey); ok {
				if span, ok := v.(trace.Span); ok {
					if tx.Error != nil {
						span.SetStatus(codes.Error, tx.Error.Error())
					}
					span.End()
				}
			}
		}
	}

	_ = db.Callback().Create().Before("gorm:create").Register("obs:before_create", before("create"))
	_ = db.Callback().Create().After("gorm:create").Register("obs:after_create", observe("create"))
	_ = db.Callback().Query().Before("gorm:query").Register("obs:before_query", before("query"))
	_ = db.Callback().Query().After("gorm:query").Register("obs:after_query", observe("query"))
	_ = db.Callback().Update().Before("gorm:update").Register("obs:before_update", before("update"))
	_ = db.Callback().Update().After("gorm:update").Register("obs:after_update", observe("update"))
	_ = db.Callback().Delete().Before("gorm:delete").Register("obs:before_delete", before("delete"))
	_ = db.Callback().Delete().After("gorm:delete").Register("obs:after_delete", observe("delete"))
	_ = db.Callback().Raw().Before("gorm:raw").Register("obs:before_raw", before("raw"))
	_ = db.Callback().Raw().After("gorm:raw").Register("obs:after_raw", observe("raw"))
	return db
}

func initDBMetrics() {
	dbMetricsOnce.Do(func() {
		dbQueries = mustRegisterCounterVec("db_client_queries_total", "Total DB queries", []string{"service", "env", "db_operation", "result"})
		dbLatency = mustRegisterHistogramVec("db_client_query_duration_seconds", "DB query duration", []string{"service", "env", "db_operation", "result"})
	})
}

func mustRegisterCounterVec(name, help string, labels []string) *prometheus.CounterVec {
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: help}, labels)
	if err := prometheus.Register(cv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing
			}
		}
	}
	return cv
}

func mustRegisterHistogramVec(name, help string, labels []string) *prometheus.HistogramVec {
	hv := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: name, Help: help, Buckets: prometheus.DefBuckets}, labels)
	if err := prometheus.Register(hv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
				return existing
			}
		}
	}
	return hv
}
