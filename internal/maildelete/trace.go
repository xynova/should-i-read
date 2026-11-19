package maildelete

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func startApplySpan(ctx context.Context, jobID string) (context.Context, trace.Span) {
	return startDeleteSpan(ctx, "maildelete.Apply",
		attribute.String("delete.job_id", jobID),
	)
}

func startJobSpan(ctx context.Context) (context.Context, trace.Span) {
	return startDeleteSpan(ctx, "maildelete.StartJob")
}

func startMessageDeleteSpan(ctx context.Context, jobID, objectHash, folder, envelopeID string) (context.Context, trace.Span) {
	return startDeleteSpan(ctx, "maildelete.MessageDelete",
		attribute.String("delete.job_id", jobID),
		attribute.String("delete.object_hash", objectHash),
		attribute.String("delete.folder", folder),
		attribute.String("delete.envelope_id", envelopeID),
	)
}

func setDeleteJobID(span trace.Span, jobID string) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.String("delete.job_id", jobID))
}

func setDeleteCounts(span trace.Span, failed, trashed int) {
	if span == nil {
		return
	}
	span.SetAttributes(
		attribute.Int("delete.failed", failed),
		attribute.Int("delete.trashed", trashed),
	)
}

func startDeleteSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/maildelete")
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func endDeleteSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
