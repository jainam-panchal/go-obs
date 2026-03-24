package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/jainam-panchal/go-obs/module/bootstrap"
	"github.com/jainam-panchal/go-obs/module/config"
	"github.com/jainam-panchal/go-obs/module/dbx"
	"github.com/jainam-panchal/go-obs/module/ginx"
	"github.com/jainam-panchal/go-obs/module/health"
	"github.com/jainam-panchal/go-obs/module/metrics"
	"github.com/jainam-panchal/go-obs/module/workerx"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	taskTypeDemo = "demo:observability"
)

type demoPayload struct {
	TriggerSource string `json:"trigger_source"`
	TenantID      string `json:"tenant_id,omitempty"`
	TraceID       string `json:"trace_id"`
	QueuedAt      string `json:"queued_at"`
}

type demoRow struct {
	ID        uint      `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	Name      string    `gorm:"size:64;not null"`
}

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_server_requests_total",
		Help: "Total HTTP requests",
	}, []string{"service", "env", "method", "route", "status_class"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_request_duration_seconds",
		Help:    "HTTP request latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "env", "method", "route", "status_class"})

	httpInflight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_server_inflight_requests",
		Help: "In-flight HTTP requests",
	}, []string{"service", "env"})
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	rt, err := bootstrap.Init(ctx, cfg)
	if err != nil {
		log.Fatalf("bootstrap init: %v", err)
	}
	defer func() {
		_ = bootstrap.Shutdown(context.Background(), rt)
	}()

	redisAddr := getenv("REDIS_ADDR", "127.0.0.1:16379")
	queueName := getenv("ASYNQ_QUEUE", "default")
	httpAddr := getenv("HTTP_ADDR", ":18080")
	dbPath := getenv("DEMO_SQLITE_PATH", "file:demo-observability.db?cache=shared&_busy_timeout=5000")

	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	gdb = dbx.WrapGORM(gdb, rt)
	if err := gdb.AutoMigrate(&demoRow{}); err != nil {
		log.Fatalf("migrate sqlite: %v", err)
	}

	redisOpt := asynq.RedisClientOpt{Addr: redisAddr}
	client := asynq.NewClient(redisOpt)
	defer client.Close()
	workerx.ObserveQueueSnapshot(rt, queueName, taskTypeDemo, 0, 0, 0)

	mux := asynq.NewServeMux()
	mux.Use(workerx.AsynqMiddleware(rt))
	mux.HandleFunc(taskTypeDemo, func(taskCtx context.Context, task *asynq.Task) error {
		return processDemoTask(taskCtx, task)
	})

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 5,
		Queues: map[string]int{
			queueName: 1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(taskCtx context.Context, task *asynq.Task, err error) {
			logJSON(map[string]any{
				"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				"level":     "error",
				"msg":       "job_failed",
				"service":   cfg.ServiceName,
				"env":       cfg.DeploymentEnv,
				"trace_id":  extractTraceID(task.Payload()),
				"task_type": task.Type(),
				"queue":     queueName,
				"error":     err.Error(),
			})
		}),
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Run(mux); err != nil {
			log.Printf("asynq server stopped: %v", err)
		}
	}()

	go startQueueReporter(ctx, redisOpt, rt, queueName, taskTypeDemo)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginx.Middleware(rt))
	r.Use(httpTelemetryMiddleware(cfg.ServiceName, cfg.DeploymentEnv))

	health.RegisterRoutes(r)
	metrics.RegisterRoute(r)

	r.POST("/jobs/demo", func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-Id")
		if traceID == "" {
			traceID = strings.ReplaceAll(uuid.NewString(), "-", "")
		}

		payload := demoPayload{
			TriggerSource: "http",
			TenantID:      c.Query("tenant_id"),
			TraceID:       traceID,
			QueuedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		}
		b, _ := json.Marshal(payload)
		task := asynq.NewTask(taskTypeDemo, b)

		info, err := client.Enqueue(task, asynq.Queue(queueName), asynq.MaxRetry(3), asynq.Timeout(30*time.Second))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		workerx.ObserveEnqueue(rt, queueName, taskTypeDemo)
		logJSON(map[string]any{
			"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
			"level":            "info",
			"msg":              "job_queued",
			"service":          cfg.ServiceName,
			"env":              cfg.DeploymentEnv,
			"job_execution_id": uuid.NewString(),
			"asynq_job_id":     info.ID,
			"task_type":        taskTypeDemo,
			"queue":            queueName,
			"attempt":          0,
			"tenant_id":        payload.TenantID,
			"trigger_source":   payload.TriggerSource,
			"trace_id":         payload.TraceID,
		})

		c.JSON(http.StatusAccepted, gin.H{"id": info.ID, "trace_id": payload.TraceID})
	})

	r.GET("/db/ping", func(c *gin.Context) {
		row := demoRow{Name: "ping"}
		if err := gdb.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_error", "error": err.Error()})
			return
		}
		var count int64
		if err := gdb.WithContext(c.Request.Context()).Model(&demoRow{}).Count(&count).Error; err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_error", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "rows": count})
	})

	httpSrv := &http.Server{Addr: httpAddr, Handler: r}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server stopped: %v", err)
		}
	}()

	log.Printf("example service running on %s with redis %s", httpAddr, redisAddr)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	srv.Shutdown()
	wg.Wait()
}

func processDemoTask(ctx context.Context, task *asynq.Task) error {
	var payload demoPayload
	_ = json.Unmarshal(task.Payload(), &payload)
	if payload.TraceID == "" {
		payload.TraceID = strings.ReplaceAll(uuid.NewString(), "-", "")
	}

	_ = ctx
	time.Sleep(200 * time.Millisecond)
	return nil
}

func startQueueReporter(ctx context.Context, redisOpt asynq.RedisClientOpt, rt *bootstrap.Runtime, queue, taskType string) {
	inspector := asynq.NewInspector(redisOpt)
	defer inspector.Close()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := inspector.GetQueueInfo(queue)
			if err != nil {
				continue
			}

			workerx.ObserveQueueSnapshot(rt, queue, taskType, info.Size, info.Latency.Seconds(), float64(info.Archived))
		}
	}
}

func httpTelemetryMiddleware(service, env string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		httpInflight.WithLabelValues(service, env).Inc()
		defer httpInflight.WithLabelValues(service, env).Dec()

		c.Next()

		statusClass := fmt.Sprintf("%dxx", c.Writer.Status()/100)
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		httpRequestsTotal.WithLabelValues(service, env, c.Request.Method, route, statusClass).Inc()
		httpRequestDuration.WithLabelValues(service, env, c.Request.Method, route, statusClass).Observe(time.Since(start).Seconds())

		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		traceID := c.GetHeader("X-Trace-Id")
		if traceID == "" {
			traceID = strings.ReplaceAll(uuid.NewString(), "-", "")
		}

		logJSON(map[string]any{
			"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
			"level":       "info",
			"msg":         "http_request",
			"service":     service,
			"env":         env,
			"trace_id":    traceID,
			"span_id":     traceID[0:16],
			"request_id":  requestID,
			"route":       route,
			"status_code": c.Writer.Status(),
			"latency_ms":  strconv.FormatInt(time.Since(start).Milliseconds(), 10),
		})
	}
}

func extractTraceID(payload []byte) string {
	var p demoPayload
	_ = json.Unmarshal(payload, &p)
	return p.TraceID
}

func logJSON(fields map[string]any) {
	b, _ := json.Marshal(fields)
	log.Println(string(b))
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
