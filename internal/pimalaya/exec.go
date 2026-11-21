package pimalaya

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	pimalayaRetryMax        = 2
	pimalayaRetryBackoff    = 200 * time.Millisecond
	pimalayaRetryBackoffMax = 2 * time.Second
	pimalayaBreakerFail     = 5
	pimalayaBreakerDelay    = 30 * time.Second
)

func defaultPimalayaRetry() retrypolicy.RetryPolicy[*Result] {
	return retrypolicy.NewBuilder[*Result]().
		HandleIf(func(_ *Result, err error) bool { return isRetryablePimalayaErr(err) }).
		AbortIf(func(_ *Result, err error) bool {
			return errors.Is(err, context.Canceled) || !isRetryablePimalayaErr(err)
		}).
		WithMaxRetries(pimalayaRetryMax).
		WithBackoff(pimalayaRetryBackoff, pimalayaRetryBackoffMax).
		WithJitterFactor(0.2).
		ReturnLastFailure().
		Build()
}

func defaultPimalayaBreaker() circuitbreaker.CircuitBreaker[*Result] {
	return circuitbreaker.NewBuilder[*Result]().
		HandleIf(func(_ *Result, err error) bool { return isRetryablePimalayaErr(err) }).
		WithFailureThreshold(pimalayaBreakerFail).
		WithDelay(pimalayaBreakerDelay).
		Build()
}

func isRetryablePimalayaErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if code, ok := sirerr.AsCode(err); ok {
		switch code {
		case sirerr.CodeUnavailable:
			return true
		case sirerr.CodeNeedsReview, sirerr.CodeInvalid, sirerr.CodeNotFound,
			sirerr.CodeFailed, sirerr.CodeAI, sirerr.CodeAuth, sirerr.CodeNetwork:
			return false
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	lower := strings.ToLower(err.Error())
	needles := []string{
		"signal: killed",
		"killed",
		"connection reset",
		"i/o timeout",
		"timeout",
		"eof",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}
