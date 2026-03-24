package workerx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/peralta/go-observability-kit/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel/trace"
)

func TestAsynqMiddlewareCallsNext(t *testing.T) {
	mw := AsynqMiddleware(&bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}})

	called := false
	h := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		called = true
		return nil
	}))

	if err := h.ProcessTask(context.Background(), asynq.NewTask("demo", []byte(`{"trace_id":"1234567890abcdef1234567890abcdef"}`))); err != nil {
		t.Fatalf("ProcessTask error = %v", err)
	}
	if !called {
		t.Fatal("expected next handler to be called")
	}
}

func TestAsynqMiddlewareStableExecutionIDAndTrace(t *testing.T) {
	mw := AsynqMiddleware(&bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}})

	buf := &bytes.Buffer{}
	origWriter := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origWriter)
		log.SetFlags(origFlags)
	}()

	h := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return nil }))
	traceID := "1234567890abcdef1234567890abcdef"
	parentSpanID := "1122334455667788"
	traceParent := "00-" + traceID + "-" + parentSpanID + "-01"
	if err := h.ProcessTask(context.Background(), asynq.NewTask("demo", []byte(`{"trace_id":"`+traceID+`","span_id":"`+parentSpanID+`","traceparent":"`+traceParent+`"}`))); err != nil {
		t.Fatalf("ProcessTask error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected lifecycle logs, got: %q", buf.String())
	}

	started := map[string]any{}
	succeeded := map[string]any{}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "{") {
			m := map[string]any{}
			if err := json.Unmarshal([]byte(ln), &m); err == nil {
				if m["msg"] == "job_started" {
					started = m
				}
				if m["msg"] == "job_succeeded" {
					succeeded = m
				}
			}
		}
	}
	if started["job_execution_id"] == nil || succeeded["job_execution_id"] == nil {
		t.Fatalf("missing execution IDs in logs: %s", buf.String())
	}
	if started["job_execution_id"] != succeeded["job_execution_id"] {
		t.Fatalf("execution IDs differ: start=%v success=%v", started["job_execution_id"], succeeded["job_execution_id"])
	}
	if started["trace_id"] != traceID || succeeded["trace_id"] != traceID {
		t.Fatalf("trace IDs not correlated: start=%v success=%v", started["trace_id"], succeeded["trace_id"])
	}
}

func TestAsynqMiddlewareRetryMetricNotIncrementedWithoutRetryMetadata(t *testing.T) {
	mw := AsynqMiddleware(&bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}})
	before := testutil.ToFloat64(jobsRetriedTotal.WithLabelValues("svc", "dev", "default", "demo"))

	h := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return errors.New("boom") }))
	_ = h.ProcessTask(context.Background(), asynq.NewTask("demo", []byte(`{}`)))

	after := testutil.ToFloat64(jobsRetriedTotal.WithLabelValues("svc", "dev", "default", "demo"))
	if after != before {
		t.Fatalf("expected retry metric unchanged without retry metadata, before=%v after=%v", before, after)
	}
}

func TestObserveQueueMetricsHelpers(t *testing.T) {
	rt := &bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}}
	ObserveEnqueue(rt, "critical", "demo")
	ObserveRetried(rt, "critical", "demo")
	ObserveQueueSnapshot(rt, "critical", "demo", 7, 12.5, 3)

	if got := testutil.ToFloat64(jobsEnqueuedTotal.WithLabelValues("svc", "dev", "critical", "demo")); got < 1 {
		t.Fatalf("expected enqueued metric to increase, got=%v", got)
	}
	if got := testutil.ToFloat64(queueDepth.WithLabelValues("svc", "dev", "critical")); got != 7 {
		t.Fatalf("unexpected queue depth, got=%v", got)
	}
	if got := testutil.ToFloat64(queueOldestAge.WithLabelValues("svc", "dev", "critical")); got != 12.5 {
		t.Fatalf("unexpected oldest age, got=%v", got)
	}
	if got := testutil.ToFloat64(deadLetterTotal.WithLabelValues("svc", "dev", "critical", "demo")); got != 3 {
		t.Fatalf("unexpected dead-letter value, got=%v", got)
	}
	if got := testutil.ToFloat64(jobsRetriedTotal.WithLabelValues("svc", "dev", "critical", "demo")); got < 1 {
		t.Fatalf("unexpected retried value, got=%v", got)
	}
}

func TestAsynqMiddlewareDoesNotMutateDeadLetterGauge(t *testing.T) {
	rt := &bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}}
	queue := "dlq-semantics"
	taskType := "demo-fail"
	ObserveQueueSnapshot(rt, queue, taskType, 1, 1, 3)

	mw := AsynqMiddleware(rt)
	h := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return errors.New("boom") }))
	_ = h.ProcessTask(context.Background(), asynq.NewTask(taskType, []byte(`{}`)))

	if got := testutil.ToFloat64(deadLetterTotal.WithLabelValues("svc", "dev", queue, taskType)); got != 3 {
		t.Fatalf("dead-letter gauge should remain snapshot-driven, got=%v", got)
	}
}

func TestAsynqMiddlewareMetricsAcrossMultipleRuns(t *testing.T) {
	rt := &bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}}
	mw := AsynqMiddleware(rt)

	queue := "metrics-run"
	taskType := "demo-run"
	ObserveQueueSnapshot(rt, queue, taskType, 4, 9.5, 2)

	hSuccess := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return nil }))
	hFailure := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return errors.New("boom") }))

	for i := 0; i < 3; i++ {
		if err := hSuccess.ProcessTask(context.Background(), asynq.NewTask(taskType, []byte(`{}`))); err != nil {
			t.Fatalf("unexpected success handler error: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		_ = hFailure.ProcessTask(context.Background(), asynq.NewTask(taskType, []byte(`{}`)))
	}

	started := testutil.ToFloat64(jobsStartedTotal.WithLabelValues("svc", "dev", "default", taskType))
	succeeded := testutil.ToFloat64(jobsSucceededTotal.WithLabelValues("svc", "dev", "default", taskType))
	failed := testutil.ToFloat64(jobsFailedTotal.WithLabelValues("svc", "dev", "default", taskType))
	retried := testutil.ToFloat64(jobsRetriedTotal.WithLabelValues("svc", "dev", "default", taskType))

	if started < 5 || math.Abs((succeeded+failed)-started) > 0.001 {
		t.Fatalf("invalid lifecycle totals started=%v succeeded=%v failed=%v", started, succeeded, failed)
	}
	if retried != 0 {
		t.Fatalf("expected retried_total to stay 0 without retry metadata, got=%v", retried)
	}
	if got := testutil.ToFloat64(deadLetterTotal.WithLabelValues("svc", "dev", queue, taskType)); got != 2 {
		t.Fatalf("dead-letter snapshot drifted unexpectedly, got=%v", got)
	}
	if got := testutil.ToFloat64(queueDepth.WithLabelValues("svc", "dev", queue)); got != 4 {
		t.Fatalf("queue depth snapshot drifted unexpectedly, got=%v", got)
	}
}

func TestParseTraceParentSampledBitSemantics(t *testing.T) {
	sc, err := parseTraceParent("00-1234567890abcdef1234567890abcdef-1122334455667788-03")
	if err != nil {
		t.Fatalf("parseTraceParent error: %v", err)
	}
	if sc.TraceFlags()&trace.FlagsSampled != trace.FlagsSampled {
		t.Fatalf("expected sampled flag from 0x03, got=%v", sc.TraceFlags())
	}

	sc, err = parseTraceParent("00-1234567890abcdef1234567890abcdef-1122334455667788-02")
	if err != nil {
		t.Fatalf("parseTraceParent error: %v", err)
	}
	if sc.TraceFlags()&trace.FlagsSampled == trace.FlagsSampled {
		t.Fatalf("did not expect sampled flag from 0x02, got=%v", sc.TraceFlags())
	}
}

func TestWorkerQueueTaskLabelCardinalityStabilityUnderLoad(t *testing.T) {
	rt := &bootstrap.Runtime{Config: config.Config{ServiceName: "svc-worker-card", DeploymentEnv: "bench"}}
	mw := AsynqMiddleware(rt)
	h := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return nil }))

	for i := 0; i < 500; i++ {
		payload := `{"tenant_id":"tenant-` + strconv.Itoa(i) + `","trace_id":"1234567890abcdef1234567890abcdef"}`
		if err := h.ProcessTask(context.Background(), asynq.NewTask("demo:cardinality", []byte(payload))); err != nil {
			t.Fatalf("process task: %v", err)
		}
	}

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	seriesCount := 0
	for _, mf := range mfs {
		if mf.GetName() != "asynq_jobs_started_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range metric.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["service"] == "svc-worker-card" && labels["env"] == "bench" {
				if labels["queue"] != "default" || labels["task_type"] != "demo:cardinality" {
					t.Fatalf("unexpected queue/task labels: queue=%s task_type=%s", labels["queue"], labels["task_type"])
				}
				seriesCount++
			}
		}
	}
	if seriesCount != 1 {
		t.Fatalf("expected one stable series for queue/task labels, got=%d", seriesCount)
	}
}
