package workerx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/peralta/go-observability-kit/config"
	"github.com/prometheus/client_golang/prometheus/testutil"
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
	if err := h.ProcessTask(context.Background(), asynq.NewTask("demo", []byte(`{"trace_id":"`+traceID+`"}`))); err != nil {
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
