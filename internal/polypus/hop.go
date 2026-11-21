package polypus

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	chatRetryMax        = 1
	chatRetryBackoff    = 2 * time.Second
	chatRetryBackoffMax = 2 * time.Second
	chatBreakerFail     = 8
	chatBreakerDelay    = 30 * time.Second
)

func requireDeadline(ctx context.Context, op string) error {
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return sirerr.New(sirerr.CodeInvalid, op, "missing deadline")
	}
	return nil
}

func hopSpan(ctx context.Context, name, model, route string) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/polypus")
	attrs := []attribute.KeyValue{
		attribute.String("http.route", route),
	}
	if strings.TrimSpace(model) != "" {
		attrs = append(attrs, attribute.String("llm.model", model))
	}
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func endHopSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func hopRetryPolicy[R any]() retrypolicy.RetryPolicy[R] {
	return retrypolicy.NewBuilder[R]().
		HandleIf(func(_ R, err error) bool { return isRetryableHop(err) }).
		AbortIf(func(_ R, err error) bool {
			return errors.Is(err, context.Canceled) || !isRetryableHop(err)
		}).
		WithMaxRetries(chatRetryMax).
		WithBackoff(chatRetryBackoff, chatRetryBackoffMax).
		WithJitterFactor(0.2).
		ReturnLastFailure().
		Build()
}

func hopBreaker[R any]() circuitbreaker.CircuitBreaker[R] {
	return circuitbreaker.NewBuilder[R]().
		HandleIf(func(_ R, err error) bool { return isRetryableHop(err) }).
		WithFailureThreshold(chatBreakerFail).
		WithDelay(chatBreakerDelay).
		Build()
}

func runHop[R any](ctx context.Context, inner func() (R, error)) (R, error) {
	return failsafe.With(hopBreaker[R](), hopRetryPolicy[R]()).WithContext(ctx).Get(inner)
}

func isRetryableHop(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if code, ok := sirerr.AsCode(err); ok && code == sirerr.CodeInvalid {
		return false
	}
	if code, ok := sirerr.AsCode(err); ok && code == sirerr.CodeUnavailable {
		return true
	}
	lower := strings.ToLower(err.Error())
	for _, n := range []string{"429", "502", "503", "504", "timeout", "connection reset"} {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}
