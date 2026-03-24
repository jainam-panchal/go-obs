package module_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jainam-panchal/go-obs/module/bootstrap"
	"github.com/jainam-panchal/go-obs/module/config"
	"github.com/jainam-panchal/go-obs/module/ginx"
	"github.com/jainam-panchal/go-obs/module/workerx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceContinuityHTTPEnqueueWorker(t *testing.T) {
	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(prevProp)

	rt := &bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginx.Middleware(rt))

	var payload []byte
	r.POST("/enqueue", func(c *gin.Context) {
		sc := trace.SpanContextFromContext(c.Request.Context())
		p := map[string]any{
			"trace_id":    sc.TraceID().String(),
			"span_id":     sc.SpanID().String(),
			"traceparent": "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-01",
		}
		payload, _ = json.Marshal(p)
		c.Status(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodPost, "/enqueue", nil)
	req.Header.Set("traceparent", "00-1234567890abcdef1234567890abcdef-1122334455667788-01")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("enqueue status=%d", w.Code)
	}
	httpTraceID := w.Header().Get("X-Trace-Id")
	if httpTraceID == "" {
		t.Fatal("missing X-Trace-Id from HTTP middleware")
	}

	buf := &bytes.Buffer{}
	origWriter := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origWriter)
		log.SetFlags(origFlags)
	}()

	mw := workerx.AsynqMiddleware(rt)
	h := mw(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error { return nil }))
	if err := h.ProcessTask(context.Background(), asynq.NewTask("demo:trace", payload)); err != nil {
		t.Fatalf("worker handler error: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) == 0 {
		t.Fatal("expected worker logs")
	}

	var started map[string]any
	for _, ln := range lines {
		m := map[string]any{}
		if err := json.Unmarshal(ln, &m); err == nil && m["msg"] == "job_started" {
			started = m
			break
		}
	}
	if started == nil {
		t.Fatalf("job_started log not found: %s", buf.String())
	}
	if got, _ := started["trace_id"].(string); got != httpTraceID {
		t.Fatalf("trace continuity mismatch: http=%s worker=%s", httpTraceID, got)
	}
}
