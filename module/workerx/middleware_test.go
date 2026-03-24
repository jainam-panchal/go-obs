package workerx

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/peralta/go-observability-kit/bootstrap"
	"github.com/peralta/go-observability-kit/config"
)

func TestAsynqMiddlewareCallsNext(t *testing.T) {
	mw := AsynqMiddleware(&bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}})

	called := false
	h := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		called = true
		return nil
	}))

	if err := h.ProcessTask(context.Background(), asynq.NewTask("demo", []byte(`{"trace_id":"t1"}`))); err != nil {
		t.Fatalf("ProcessTask error = %v", err)
	}
	if !called {
		t.Fatal("expected next handler to be called")
	}
}
