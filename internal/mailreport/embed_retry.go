package mailreport

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	// One attempt: stropEmbedHTTPTimeout is minutes-long for catalog batches.
	// Extra retries would exceed operateTimeoutAttach.
	embedRetryMax        = 0
	embedRetryBackoff    = 4 * time.Second
	embedRetryBackoffMax = 8 * time.Second
	embedBreakerFail     = 8
	embedBreakerDelay    = 30 * time.Second
)

// RetryingEmbedderConfig wraps an embedder with failsafe retry and a breaker.
type RetryingEmbedderConfig struct {
	Inner   harness.Embedder
	Retry   retrypolicy.RetryPolicy[[][]float64]
	Breaker circuitbreaker.CircuitBreaker[[][]float64]
}

// CreateRetryingEmbedder wraps inner with a breaker. Default retry count is zero
// (catalog batches already use a minutes-long HTTP budget). Tests may inject retries.
func CreateRetryingEmbedder(cfg RetryingEmbedderConfig) (harness.Embedder, error) {
	const op = "mailreport.CreateRetryingEmbedder"
	if cfg.Inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil inner embedder")
	}
	retry := cfg.Retry
	if retry == nil {
		retry = defaultEmbedRetryPolicy()
	}
	breaker := cfg.Breaker
	if breaker == nil {
		breaker = defaultEmbedBreaker()
	}
	return &retryingEmbedder{inner: cfg.Inner, retry: retry, breaker: breaker}, nil
}

type retryingEmbedder struct {
	inner   harness.Embedder
	retry   retrypolicy.RetryPolicy[[][]float64]
	breaker circuitbreaker.CircuitBreaker[[][]float64]
}

func (r *retryingEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	const op = "mailreport.retryingEmbedder.Embed"
	if r == nil || r.inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil embedder")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "embed canceled")
	}
	innerFn := func() ([][]float64, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return r.inner.Embed(ctx, texts)
	}
	// Breaker is outer: one Embed() is one sample, not each inner 503 POST.
	vecs, err := failsafe.With(r.breaker).WithContext(ctx).Get(func() ([][]float64, error) {
		return failsafe.With(r.retry).WithContext(ctx).Get(innerFn)
	})
	if err != nil {
		code := sirerr.CodeAI
		if isRetryableEmbedErr(err) || embedRetryAlreadyExhausted(err) {
			code = sirerr.CodeUnavailable
		}
		return nil, sirerr.Wrap(err, code, op, "embed after retries")
	}
	return vecs, nil
}

func defaultEmbedRetryPolicy() retrypolicy.RetryPolicy[[][]float64] {
	return retrypolicy.NewBuilder[[][]float64]().
		HandleIf(func(_ [][]float64, err error) bool { return isRetryableEmbedErr(err) }).
		AbortIf(func(_ [][]float64, err error) bool {
			return errors.Is(err, context.Canceled) || embedRetryAlreadyExhausted(err)
		}).
		WithMaxRetries(embedRetryMax).
		WithBackoff(embedRetryBackoff, embedRetryBackoffMax).
		WithJitterFactor(0.2).
		ReturnLastFailure().
		Build()
}

func defaultEmbedBreaker() circuitbreaker.CircuitBreaker[[][]float64] {
	return circuitbreaker.NewBuilder[[][]float64]().
		HandleIf(func(_ [][]float64, err error) bool {
			return isRetryableEmbedErr(err) || embedRetryAlreadyExhausted(err)
		}).
		WithFailureThreshold(embedBreakerFail).
		WithDelay(embedBreakerDelay).
		Build()
}

func embedRetryAlreadyExhausted(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "retries exceeded")
}

func isRetryableEmbedErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if embedRetryAlreadyExhausted(err) {
		return false
	}
	if code, ok := sirerr.AsCode(err); ok && code == sirerr.CodeInvalid {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	lower := strings.ToLower(err.Error())
	needles := []string{
		"503",
		"429",
		"502",
		"504",
		"service unavailable",
		"timeout",
		"awaiting headers",
		"connection reset",
		"i/o timeout",
		"eof",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	if code, ok := sirerr.AsCode(err); ok && code == sirerr.CodeUnavailable {
		return true
	}
	return false
}
