package polypus

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TimeoutHeader is the per-request Polypus hop override (duration or integer seconds).
const TimeoutHeader = "X-Polypus-Timeout"

// embedHopSlack keeps the gateway hop slightly inside the host context deadline
// so Polypus can return 504 before the client cancels.
const embedHopSlack = time.Second

type embedTimeoutTransport struct {
	inner http.RoundTripper
}

// InstrumentEmbedTimeoutHeader wraps client.Transport so embeddings POSTs send
// TimeoutHeader from the request context deadline. Idempotent.
func InstrumentEmbedTimeoutHeader(client *http.Client) {
	if client == nil {
		return
	}
	if _, ok := client.Transport.(*embedTimeoutTransport); ok {
		return
	}
	inner := client.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}
	client.Transport = &embedTimeoutTransport{inner: inner}
}

func (t *embedTimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.inner == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	if req != nil && isEmbeddingsPath(req.URL) && strings.TrimSpace(req.Header.Get(TimeoutHeader)) == "" {
		if hop, ok := hopTimeoutFromContext(req.Context()); ok {
			req = req.Clone(req.Context())
			req.Header.Set(TimeoutHeader, hop.String())
		}
	}
	return t.inner.RoundTrip(req)
}

func applyTimeoutHeader(req *http.Request) {
	if req == nil || strings.TrimSpace(req.Header.Get(TimeoutHeader)) != "" {
		return
	}
	if !isEmbeddingsPath(req.URL) {
		return
	}
	hop, ok := hopTimeoutFromContext(req.Context())
	if !ok {
		return
	}
	req.Header.Set(TimeoutHeader, hop.String())
}

func hopTimeoutFromContext(ctx context.Context) (time.Duration, bool) {
	if ctx == nil {
		return 0, false
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0, false
	}
	hop := remaining - embedHopSlack
	if hop < time.Second {
		hop = remaining
	}
	if hop < time.Second {
		return time.Second, true
	}
	return hop.Truncate(time.Millisecond), true
}

func isEmbeddingsPath(u *url.URL) bool {
	if u == nil {
		return false
	}
	return strings.Contains(u.Path, "/embeddings")
}
