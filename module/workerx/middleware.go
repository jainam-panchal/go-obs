package workerx

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/peralta/go-observability-kit/bootstrap"
)

// AsynqMiddleware is a pass-through skeleton for job instrumentation.
func AsynqMiddleware(_ *bootstrap.Runtime) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			return next.ProcessTask(ctx, task)
		})
	}
}
