package maildelete

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func startJobSpan(ctx context.Context) (context.Context, trace.Span) {
	return startDeleteSpan(ctx, "maildelete.StartJob")
}

func setDeleteJobID(span trace.Span, jobID string) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.String("delete.job_id", jobID))
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
