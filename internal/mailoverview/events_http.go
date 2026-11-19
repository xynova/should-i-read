package mailoverview

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const eventsHeartbeat = 15 * time.Second

func (s *IntentPageServer) serveEvents(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, ": connected\n\n"); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "write connected")
		return
	}
	flusher.Flush()
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		span.RecordError(err)
	}

	ch := s.Events.Subscribe(ctx)
	ticker := time.NewTicker(eventsHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				span.RecordError(err)
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "write event")
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "write ping")
				return
			}
			flusher.Flush()
		}
	}
}
