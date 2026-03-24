package module_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/peralta/go-observability-kit/config"
	"github.com/peralta/go-observability-kit/dbx"
	"github.com/peralta/go-observability-kit/ginx"
	"github.com/peralta/go-observability-kit/workerx"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type metricContractModel struct {
	ID   uint
	Name string
}

func TestRequiredMetricFamiliesAreEmittedByModuleAPIs(t *testing.T) {
	rt := &bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginx.Middleware(rt))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("http middleware route status=%d", w.Code)
	}

	workerx.ObserveEnqueue(rt, "default", "demo:metric-contract")
	workerx.ObserveRetried(rt, "default", "demo:metric-contract")
	workerx.ObserveQueueSnapshot(rt, "default", "demo:metric-contract", 3, 4.5, 1)
	mw := workerx.AsynqMiddleware(rt)
	success := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return nil }))
	failure := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return context.DeadlineExceeded }))
	_ = success.ProcessTask(context.Background(), asynq.NewTask("demo:metric-contract", []byte(`{}`)))
	_ = failure.ProcessTask(context.Background(), asynq.NewTask("demo:metric-contract", []byte(`{}`)))

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db = dbx.WrapGORM(db, rt)
	if err := db.AutoMigrate(&metricContractModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&metricContractModel{Name: "a"}).Error; err != nil {
		t.Fatalf("db create: %v", err)
	}
	var out metricContractModel
	if err := db.First(&out).Error; err != nil {
		t.Fatalf("db query: %v", err)
	}

	required := []string{
		"http_server_requests_total",
		"http_server_request_duration_seconds",
		"http_server_inflight_requests",
		"asynq_jobs_enqueued_total",
		"asynq_jobs_started_total",
		"asynq_jobs_succeeded_total",
		"asynq_jobs_failed_total",
		"asynq_jobs_retried_total",
		"asynq_job_duration_seconds",
		"asynq_queue_depth",
		"asynq_queue_oldest_age_seconds",
		"asynq_dead_letter_total",
		"db_client_queries_total",
		"db_client_query_duration_seconds",
	}

	names := gatherMetricFamilyNames(t)
	for _, name := range required {
		if !names[name] {
			t.Fatalf("required metric family not found: %s", name)
		}
	}
}

func gatherMetricFamilyNames(t *testing.T) map[string]bool {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	out := make(map[string]bool, len(mfs))
	for _, mf := range mfs {
		out[mf.GetName()] = true
	}
	return out
}
