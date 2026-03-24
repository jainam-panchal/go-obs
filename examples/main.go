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

	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/peralta/go-observability-kit/config"
	"github.com/peralta/go-observability-kit/ginx"
	"github.com/peralta/go-observability-kit/health"
	"github.com/peralta/go-observability-kit/metrics"
	"github.com/peralta/go-observability-kit/workerx"
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

	jobsEnqueuedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "asynq_jobs_enqueued_total",
		Help: "Total enqueued jobs",
	}, []string{"service", "env", "queue", "task_type"})

	jobsStartedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "asynq_jobs_started_total",
		Help: "Total started jobs",
	}, []string{"service", "env", "queue", "task_type"})

	jobsSucceededTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "asynq_jobs_succeeded_total",
		Help: "Total succeeded jobs",
	}, []string{"service", "env", "queue", "task_type"})

	jobsFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "asynq_jobs_failed_total",
		Help: "Total failed jobs",
	}, []string{"service", "env", "queue", "task_type"})

	jobsRetriedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "asynq_jobs_retried_total",
		Help: "Total retried jobs",
	}, []string{"service", "env", "queue", "task_type"})

	jobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "asynq_job_duration_seconds",
		Help:    "Job processing duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "env", "queue", "task_type", "result"})

	queueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "asynq_queue_depth",
		Help: "Queue depth",
	}, []string{"service", "env", "queue"})

	queueOldestAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "asynq_queue_oldest_age_seconds",
		Help: "Oldest job age in queue",
	}, []string{"service", "env", "queue"})

	deadLetterTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "asynq_dead_letter_total",
		Help: "Dead lettered jobs",
	}, []string{"service", "env", "queue", "task_type"})
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

	redisOpt := asynq.RedisClientOpt{Addr: redisAddr}
	client := asynq.NewClient(redisOpt)
	defer client.Close()
	queueDepth.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queueName).Set(0)
	queueOldestAge.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queueName).Set(0)
	deadLetterTotal.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queueName, taskTypeDemo).Set(0)

	mux := asynq.NewServeMux()
	mux.Use(workerx.AsynqMiddleware(rt))
	mux.HandleFunc(taskTypeDemo, func(taskCtx context.Context, task *asynq.Task) error {
		return processDemoTask(taskCtx, task, cfg, queueName)
	})

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 5,
		Queues: map[string]int{
			queueName: 1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(taskCtx context.Context, task *asynq.Task, err error) {
			jobsFailedTotal.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queueName, task.Type()).Inc()
			jobsRetriedTotal.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queueName, task.Type()).Inc()
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

	go startQueueReporter(ctx, redisOpt, cfg.ServiceName, cfg.DeploymentEnv, queueName)

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

		jobsEnqueuedTotal.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queueName, taskTypeDemo).Inc()
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

func processDemoTask(ctx context.Context, task *asynq.Task, cfg config.Config, queue string) error {
	started := time.Now()
	retryCount, ok := asynq.GetRetryCount(ctx)
	if !ok {
		retryCount = 0
	}
	attempt := retryCount + 1
	jobID, ok := asynq.GetTaskID(ctx)
	if !ok || jobID == "" {
		jobID = uuid.NewString()
	}

	var payload demoPayload
	_ = json.Unmarshal(task.Payload(), &payload)
	if payload.TraceID == "" {
		payload.TraceID = strings.ReplaceAll(uuid.NewString(), "-", "")
	}

	jobExecutionID := uuid.NewString()
	jobsStartedTotal.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queue, task.Type()).Inc()
	logJSON(map[string]any{
		"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
		"level":            "info",
		"msg":              "job_started",
		"service":          cfg.ServiceName,
		"env":              cfg.DeploymentEnv,
		"job_execution_id": jobExecutionID,
		"asynq_job_id":     jobID,
		"task_type":        task.Type(),
		"queue":            queue,
		"attempt":          attempt,
		"tenant_id":        payload.TenantID,
		"trigger_source":   payload.TriggerSource,
		"trace_id":         payload.TraceID,
	})

	time.Sleep(200 * time.Millisecond)

	jobsSucceededTotal.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queue, task.Type()).Inc()
	jobDuration.WithLabelValues(cfg.ServiceName, cfg.DeploymentEnv, queue, task.Type(), "success").Observe(time.Since(started).Seconds())
	logJSON(map[string]any{
		"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
		"level":            "info",
		"msg":              "job_succeeded",
		"service":          cfg.ServiceName,
		"env":              cfg.DeploymentEnv,
		"job_execution_id": jobExecutionID,
		"asynq_job_id":     jobID,
		"task_type":        task.Type(),
		"queue":            queue,
		"attempt":          attempt,
		"tenant_id":        payload.TenantID,
		"trigger_source":   payload.TriggerSource,
		"trace_id":         payload.TraceID,
	})
	return nil
}

func startQueueReporter(ctx context.Context, redisOpt asynq.RedisClientOpt, service, env, queue string) {
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

			queueDepth.WithLabelValues(service, env, queue).Set(float64(info.Size))
			queueOldestAge.WithLabelValues(service, env, queue).Set(info.Latency.Seconds())
			deadLetterTotal.WithLabelValues(service, env, queue, taskTypeDemo).Set(float64(info.Archived))
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
