package mailoverview

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/xynova/should-i-read/internal/hostevent"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const eventsHeartbeat = 15 * time.Second

func (s *eventsHTTP) serveEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s == nil || s.Events == nil {
		http.Error(w, "events unavailable", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	if ctx == nil {
		http.Error(w, "missing request context", http.StatusInternalServerError)
		return
	}
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailoverview")
	ctx, span := tracer.Start(ctx, "mailoverview.serveEvents")
	defer span.End()
	span.SetAttributes(attribute.String("http.route", "/events"))

	if !eventsWriteConnected(w, flusher, span) {
		return
	}
	eventsSetWriteDeadline(w, span)
	ch := s.Events.Subscribe(ctx)
	ticker := time.NewTicker(eventsHeartbeat)
	defer ticker.Stop()
	eventsPumpLoop(ctx, w, flusher, span, ch, ticker)
}

func eventsSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
}

func eventsWriteConnected(w http.ResponseWriter, flusher http.Flusher, span trace.Span) bool {
	eventsSSEHeaders(w)
	if _, err := fmt.Fprintf(w, ": connected\n\n"); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "write connected")
		return false
	}
	flusher.Flush()
	return true
}

func eventsSetWriteDeadline(w http.ResponseWriter, span trace.Span) {
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		span.RecordError(err)
	}
}

func eventsPumpLoop(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, span trace.Span, ch <-chan hostevent.Event, ticker *time.Ticker) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !eventsWriteData(w, flusher, span, ev) {
				return
			}
		case <-ticker.C:
			if !eventsWritePing(w, flusher, span) {
				return
			}
		}
	}
}

func eventsWriteData(w http.ResponseWriter, flusher http.Flusher, span trace.Span, ev hostevent.Event) bool {
	payload, err := json.Marshal(ev)
	if err != nil {
		span.RecordError(err)
		return true
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "write event")
		return false
	}
	flusher.Flush()
	return true
}

func eventsWritePing(w http.ResponseWriter, flusher http.Flusher, span trace.Span) bool {
	if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "write ping")
		return false
	}
	flusher.Flush()
	return true
}
