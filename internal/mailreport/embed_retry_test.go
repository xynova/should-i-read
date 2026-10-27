package mailreport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func testEmbedRetryPolicy() retrypolicy.RetryPolicy[[][]float64] {
	return retrypolicy.NewBuilder[[][]float64]().
		HandleIf(func(_ [][]float64, err error) bool { return isRetryableEmbedErr(err) }).
		WithMaxRetries(2).
		WithBackoff(time.Nanosecond, time.Microsecond).
		ReturnLastFailure().
		Build()
}

func testEmbedBreaker() circuitbreaker.CircuitBreaker[[][]float64] {
	return circuitbreaker.NewBuilder[[][]float64]().
		HandleIf(func(_ [][]float64, err error) bool { return isRetryableEmbedErr(err) }).
		WithFailureThreshold(100).
		WithDelay(time.Hour).
		Build()
}

func TestRetryingEmbedder_retriesThenSucceeds(t *testing.T) {
	inner := &countingEmbedder{
		failN:   2,
		failErr: errors.New(`retryable http status: Service Unavailable`),
	}
	emb, err := CreateRetryingEmbedder(RetryingEmbedderConfig{
		Inner:   inner,
		Retry:   testEmbedRetryPolicy(),
		Breaker: testEmbedBreaker(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := emb.Embed(ctx, []string{"hello"}); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 3 {
		t.Fatalf("calls=%d want 3", inner.calls)
	}
}

func TestRetryingEmbedder_permanentDoesNotRetry(t *testing.T) {
	inner := &countingEmbedder{err: sirerr.New(sirerr.CodeInvalid, "stub", "empty embed text")}
	emb, err := CreateRetryingEmbedder(RetryingEmbedderConfig{
		Inner:   inner,
		Retry:   testEmbedRetryPolicy(),
		Breaker: testEmbedBreaker(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := emb.Embed(ctx, []string{"hello"}); err == nil {
		t.Fatal("expected error")
	}
	if inner.calls != 1 {
		t.Fatalf("calls=%d want 1", inner.calls)
	}
}

func TestRetryingEmbedder_timeoutIsRetryable(t *testing.T) {
	inner := &countingEmbedder{err: errors.New(`Post "https://polypus.manticore-powan.ts.net/v1/embeddings": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`)}
	emb, err := CreateRetryingEmbedder(RetryingEmbedderConfig{
		Inner:   inner,
		Retry:   testEmbedRetryPolicy(),
		Breaker: testEmbedBreaker(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, embedErr := emb.Embed(ctx, []string{"hello"})
	if embedErr == nil {
		t.Fatal("expected error")
	}
	if inner.calls != 3 {
		t.Fatalf("calls=%d want 3 (1 + 2 retries)", inner.calls)
	}
	if !strings.Contains(strings.ToLower(embedErr.Error()), "awaiting headers") {
		t.Fatalf("err %v", embedErr)
	}
}

func TestIsRetryableEmbedErr(t *testing.T) {
	t.Parallel()
	if !isRetryableEmbedErr(errors.New("retryable http status: Service Unavailable")) {
		t.Fatal("503 should retry")
	}
	if isRetryableEmbedErr(sirerr.New(sirerr.CodeInvalid, "op", "empty embed text")) {
		t.Fatal("invalid should not retry")
	}
	if isRetryableEmbedErr(context.Canceled) {
		t.Fatal("canceled should not retry")
	}
	if isRetryableEmbedErr(errors.New(`retries exceeded. last error: retryable http status: Service Unavailable`)) {
		t.Fatal("inner retries exceeded should not retry again")
	}
}

func TestRetryingEmbedder_doesNotRetryWhenInnerAlreadyExceeded(t *testing.T) {
	inner := &countingEmbedder{err: errors.New(`retries exceeded. last error: retryable http status: Service Unavailable [model=deepinfra/BAAI/bge-m3]`)}
	emb, err := CreateRetryingEmbedder(RetryingEmbedderConfig{
		Inner:   inner,
		Retry:   testEmbedRetryPolicy(),
		Breaker: testEmbedBreaker(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := emb.Embed(ctx, []string{"hello"}); err == nil {
		t.Fatal("expected error")
	}
	if inner.calls != 1 {
		t.Fatalf("calls=%d want 1", inner.calls)
	}
}

func TestRetryingEmbedder_breakerCountsCompletedEmbed(t *testing.T) {
	inner := &countingEmbedder{err: errors.New("retryable http status: Service Unavailable")}
	retry := retrypolicy.NewBuilder[[][]float64]().
		HandleIf(func(_ [][]float64, err error) bool { return isRetryableEmbedErr(err) }).
		WithMaxRetries(2).
		WithBackoff(time.Nanosecond, time.Microsecond).
		ReturnLastFailure().
		Build()
	breaker := circuitbreaker.NewBuilder[[][]float64]().
		HandleIf(func(_ [][]float64, err error) bool { return isRetryableEmbedErr(err) }).
		WithFailureThreshold(2).
		WithDelay(time.Hour).
		Build()
	emb, err := CreateRetryingEmbedder(RetryingEmbedderConfig{
		Inner:   inner,
		Retry:   retry,
		Breaker: breaker,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := emb.Embed(ctx, []string{"a"}); err == nil {
		t.Fatal("expected first embed error")
	}
	if inner.calls != 3 {
		t.Fatalf("after first embed calls=%d want 3", inner.calls)
	}
	if breaker.IsOpen() {
		t.Fatal("one completed Embed must not open the breaker")
	}
	if _, err := emb.Embed(ctx, []string{"b"}); err == nil {
		t.Fatal("expected second embed error")
	}
	if !breaker.IsOpen() {
		t.Fatal("two completed Embed failures should open the breaker")
	}
	before := inner.calls
	if _, err := emb.Embed(ctx, []string{"c"}); err == nil {
		t.Fatal("expected open breaker error")
	}
	if inner.calls != before {
		t.Fatalf("open breaker called inner calls=%d want %d", inner.calls, before)
	}
}
