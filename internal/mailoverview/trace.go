package mailoverview

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func startGuessSpan(ctx context.Context, name, model string) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailoverview")
	return tracer.Start(ctx, name,
		trace.WithAttributes(
			attribute.String("polypus.model", model),
		),
	)
}

func startCartAddSpan(ctx context.Context) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailoverview")
	return tracer.Start(ctx, "mailoverview.AddCart")
}

func setCartAddCounts(span trace.Span, requested, kept, movedFromAck int) {
	if span == nil {
		return
	}
	span.SetAttributes(
		attribute.Int("cart.add_requested", requested),
		attribute.Int("cart.add_kept", kept),
		attribute.Int("cart.moved_from_ack", movedFromAck),
	)
}

func startAfterSyncSpan(ctx context.Context, runID string) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailoverview")
	return tracer.Start(ctx, "mailoverview.AfterSync",
		trace.WithAttributes(
			attribute.String("sync.run_id", runID),
		),
	)
}

func startSyncSpan(ctx context.Context, runID, account string, excludes []string) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailoverview")
	return tracer.Start(ctx, "mailoverview.RunSync",
		trace.WithAttributes(
			attribute.String("sync.run_id", runID),
			attribute.String("sync.account", account),
			attribute.StringSlice("sync.exclude_collections", excludes),
		),
	)
}

func setSyncSpanResult(span trace.Span, status string, fetch, del int, line string) {
	if span == nil {
		return
	}
	span.SetAttributes(
		attribute.String("sync.status", status),
		attribute.Int("sync.fetch", fetch),
		attribute.Int("sync.delete", del),
		attribute.String("sync.line", line),
	)
}

func endOverviewSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
