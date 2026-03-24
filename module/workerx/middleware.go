package workerx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	workerMetricsOnce sync.Once

	jobsEnqueuedTotal  *prometheus.CounterVec
	jobsStartedTotal   *prometheus.CounterVec
	jobsSucceededTotal *prometheus.CounterVec
	jobsFailedTotal    *prometheus.CounterVec
	jobsRetriedTotal   *prometheus.CounterVec
	jobDuration        *prometheus.HistogramVec
	queueDepth         *prometheus.GaugeVec
	queueOldestAge     *prometheus.GaugeVec
	deadLetterTotal    *prometheus.GaugeVec
)

// AsynqMiddleware instruments job lifecycle with metrics, logs, and trace correlation.
func AsynqMiddleware(rt *bootstrap.Runtime) asynq.MiddlewareFunc {
	initWorkerMetrics()

	service, env := serviceEnv(rt)
	tracer := otel.Tracer("go-observability-kit/workerx")

	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			startedAt := time.Now()
			queue, ok := asynq.GetQueueName(ctx)
			if !ok || queue == "" {
				queue = "default"
			}
			retryCount, ok := asynq.GetRetryCount(ctx)
			if !ok {
				retryCount = 0
			}
			attempt := retryCount + 1
			maxRetry, hasMaxRetry := asynq.GetMaxRetry(ctx)
			jobID, ok := asynq.GetTaskID(ctx)
			if !ok || jobID == "" {
				jobID = uuid.NewString()
			}
			jobExecutionID := uuid.NewString()

			payload := parsePayload(task.Payload())
			spanStartCtx := context.Background()
			if sc, ok := remoteSpanContextFromPayload(payload); ok {
				spanStartCtx = trace.ContextWithRemoteSpanContext(spanStartCtx, sc)
			}

			spanCtx, span := tracer.Start(spanStartCtx, "asynq "+task.Type())
			traceID := ""
			if sc := trace.SpanContextFromContext(spanCtx); sc.IsValid() {
				traceID = sc.TraceID().String()
			}
			if traceID == "" {
				traceID = payload.TraceID
			}
			if traceID == "" {
				traceID = strings.ReplaceAll(uuid.NewString(), "-", "")
			}

			jobsStartedTotal.WithLabelValues(service, env, queue, task.Type()).Inc()
			logJSON(map[string]any{
				"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
				"level":            "info",
				"msg":              "job_started",
				"service":          service,
				"env":              env,
				"job_execution_id": jobExecutionID,
				"asynq_job_id":     jobID,
				"task_type":        task.Type(),
				"queue":            queue,
				"attempt":          attempt,
				"tenant_id":        payload.TenantID,
				"trigger_source":   payload.TriggerSource,
				"trace_id":         traceID,
			})

			err := next.ProcessTask(spanCtx, task)
			result := "success"
			if err != nil {
				result = "error"
				jobsFailedTotal.WithLabelValues(service, env, queue, task.Type()).Inc()
				if hasMaxRetry && retryCount < maxRetry && !errors.Is(err, asynq.SkipRetry) {
					jobsRetriedTotal.WithLabelValues(service, env, queue, task.Type()).Inc()
				}
				span.SetStatus(codes.Error, err.Error())
				logJSON(map[string]any{
					"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
					"level":            "error",
					"msg":              "job_failed",
					"service":          service,
					"env":              env,
					"job_execution_id": jobExecutionID,
					"asynq_job_id":     jobID,
					"task_type":        task.Type(),
					"queue":            queue,
					"attempt":          attempt,
					"tenant_id":        payload.TenantID,
					"trigger_source":   payload.TriggerSource,
					"trace_id":         traceID,
					"error":            err.Error(),
				})
			} else {
				jobsSucceededTotal.WithLabelValues(service, env, queue, task.Type()).Inc()
				logJSON(map[string]any{
					"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
					"level":            "info",
					"msg":              "job_succeeded",
					"service":          service,
					"env":              env,
					"job_execution_id": jobExecutionID,
					"asynq_job_id":     jobID,
					"task_type":        task.Type(),
					"queue":            queue,
					"attempt":          attempt,
					"tenant_id":        payload.TenantID,
					"trigger_source":   payload.TriggerSource,
					"trace_id":         traceID,
				})
			}

			jobDuration.WithLabelValues(service, env, queue, task.Type(), result).Observe(time.Since(startedAt).Seconds())
			span.End()
			return err
		})
	}
}

// ObserveEnqueue records a job enqueue event for contract metrics.
func ObserveEnqueue(rt *bootstrap.Runtime, queue, taskType string) {
	initWorkerMetrics()
	service, env := serviceEnv(rt)
	if queue == "" {
		queue = "default"
	}
	jobsEnqueuedTotal.WithLabelValues(service, env, queue, taskType).Inc()
}

// ObserveQueueSnapshot records queue depth/age/dead-letter snapshot metrics from queue inspectors.
func ObserveQueueSnapshot(rt *bootstrap.Runtime, queue, taskType string, depth int, oldestAgeSeconds, deadLetter float64) {
	initWorkerMetrics()
	service, env := serviceEnv(rt)
	if queue == "" {
		queue = "default"
	}
	queueDepth.WithLabelValues(service, env, queue).Set(float64(depth))
	queueOldestAge.WithLabelValues(service, env, queue).Set(oldestAgeSeconds)
	deadLetterTotal.WithLabelValues(service, env, queue, taskType).Set(deadLetter)
}

type payloadFields struct {
	TriggerSource string `json:"trigger_source"`
	TenantID      string `json:"tenant_id"`
	TraceID       string `json:"trace_id"`
	SpanID        string `json:"span_id"`
	TraceParent   string `json:"traceparent"`
}

func parsePayload(raw []byte) payloadFields {
	var p payloadFields
	_ = json.Unmarshal(raw, &p)
	if p.TriggerSource == "" {
		p.TriggerSource = "worker"
	}
	return p
}

func remoteSpanContextFromPayload(p payloadFields) (trace.SpanContext, bool) {
	if strings.TrimSpace(p.TraceParent) != "" {
		if tc, err := parseTraceParent(strings.TrimSpace(p.TraceParent)); err == nil {
			return tc, true
		}
	}

	tid, err := trace.TraceIDFromHex(strings.TrimSpace(p.TraceID))
	if err != nil || !tid.IsValid() {
		return trace.SpanContext{}, false
	}
	sid, err := trace.SpanIDFromHex(strings.TrimSpace(p.SpanID))
	if err != nil || !sid.IsValid() {
		return trace.SpanContext{}, false
	}

	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	}), true
}

func parseTraceParent(tp string) (trace.SpanContext, error) {
	parts := strings.Split(tp, "-")
	if len(parts) != 4 {
		return trace.SpanContext{}, fmt.Errorf("invalid traceparent format")
	}
	if parts[0] != "00" {
		return trace.SpanContext{}, fmt.Errorf("unsupported traceparent version: %s", parts[0])
	}
	tid, err := trace.TraceIDFromHex(parts[1])
	if err != nil {
		return trace.SpanContext{}, err
	}
	sid, err := trace.SpanIDFromHex(parts[2])
	if err != nil {
		return trace.SpanContext{}, err
	}
	flags := trace.TraceFlags(0)
	if len(parts[3]) == 2 && strings.EqualFold(parts[3], "01") {
		flags = trace.FlagsSampled
	}
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: flags,
		Remote:     true,
	}), nil
}

func initWorkerMetrics() {
	workerMetricsOnce.Do(func() {
		jobsEnqueuedTotal = mustRegisterCounterVec("asynq_jobs_enqueued_total", "Total enqueued jobs", []string{"service", "env", "queue", "task_type"})
		jobsStartedTotal = mustRegisterCounterVec("asynq_jobs_started_total", "Total started jobs", []string{"service", "env", "queue", "task_type"})
		jobsSucceededTotal = mustRegisterCounterVec("asynq_jobs_succeeded_total", "Total succeeded jobs", []string{"service", "env", "queue", "task_type"})
		jobsFailedTotal = mustRegisterCounterVec("asynq_jobs_failed_total", "Total failed jobs", []string{"service", "env", "queue", "task_type"})
		jobsRetriedTotal = mustRegisterCounterVec("asynq_jobs_retried_total", "Total retried jobs", []string{"service", "env", "queue", "task_type"})
		jobDuration = mustRegisterHistogramVec("asynq_job_duration_seconds", "Job processing duration", []string{"service", "env", "queue", "task_type", "result"})
		queueDepth = mustRegisterGaugeVec("asynq_queue_depth", "Current queue depth", []string{"service", "env", "queue"})
		queueOldestAge = mustRegisterGaugeVec("asynq_queue_oldest_age_seconds", "Oldest job age in queue", []string{"service", "env", "queue"})
		deadLetterTotal = mustRegisterGaugeVec("asynq_dead_letter_total", "Dead-lettered jobs", []string{"service", "env", "queue", "task_type"})
	})
}

func serviceEnv(rt *bootstrap.Runtime) (string, string) {
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
	return service, env
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

func mustRegisterGaugeVec(name, help string, labels []string) *prometheus.GaugeVec {
	gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, labels)
	if err := prometheus.Register(gv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.GaugeVec); ok {
				return existing
			}
		}
	}
	return gv
}

func logJSON(fields map[string]any) {
	b, _ := json.Marshal(fields)
	log.Println(string(b))
}
