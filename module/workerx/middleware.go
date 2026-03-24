package workerx

import (
	"context"
	"encoding/json"
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

	jobsStartedTotal   *prometheus.CounterVec
	jobsSucceededTotal *prometheus.CounterVec
	jobsFailedTotal    *prometheus.CounterVec
	jobsRetriedTotal   *prometheus.CounterVec
	jobDuration        *prometheus.HistogramVec
)

// AsynqMiddleware instruments job lifecycle with metrics, logs, and trace correlation.
func AsynqMiddleware(rt *bootstrap.Runtime) asynq.MiddlewareFunc {
	initWorkerMetrics()

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
			if sc, ok := remoteSpanContextFromTraceID(payload.TraceID); ok {
				spanStartCtx = trace.ContextWithRemoteSpanContext(spanStartCtx, sc)
			}

			spanCtx, span := tracer.Start(spanStartCtx, "asynq "+task.Type())
			traceID := ""
			if sc := trace.SpanContextFromContext(spanCtx); sc.IsValid() {
				traceID = sc.TraceID().String()
			}
			if traceID == "" && payload.TraceID != "" {
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
				if hasMaxRetry && retryCount < maxRetry {
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

type payloadFields struct {
	TriggerSource string `json:"trigger_source"`
	TenantID      string `json:"tenant_id"`
	TraceID       string `json:"trace_id"`
}

func parsePayload(raw []byte) payloadFields {
	var p payloadFields
	_ = json.Unmarshal(raw, &p)
	if p.TriggerSource == "" {
		p.TriggerSource = "worker"
	}
	return p
}

func remoteSpanContextFromTraceID(traceID string) (trace.SpanContext, bool) {
	tid, err := trace.TraceIDFromHex(strings.TrimSpace(traceID))
	if err != nil {
		return trace.SpanContext{}, false
	}
	if !tid.IsValid() {
		return trace.SpanContext{}, false
	}
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	}), true
}

func initWorkerMetrics() {
	workerMetricsOnce.Do(func() {
		jobsStartedTotal = mustRegisterCounterVec("asynq_jobs_started_total", "Total started jobs", []string{"service", "env", "queue", "task_type"})
		jobsSucceededTotal = mustRegisterCounterVec("asynq_jobs_succeeded_total", "Total succeeded jobs", []string{"service", "env", "queue", "task_type"})
		jobsFailedTotal = mustRegisterCounterVec("asynq_jobs_failed_total", "Total failed jobs", []string{"service", "env", "queue", "task_type"})
		jobsRetriedTotal = mustRegisterCounterVec("asynq_jobs_retried_total", "Total retried jobs", []string{"service", "env", "queue", "task_type"})
		jobDuration = mustRegisterHistogramVec("asynq_job_duration_seconds", "Job processing duration", []string{"service", "env", "queue", "task_type", "result"})
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

func logJSON(fields map[string]any) {
	b, _ := json.Marshal(fields)
	log.Println(string(b))
}
