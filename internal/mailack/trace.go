package mailack

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func startApplySpan(ctx context.Context, jobID string) (context.Context, trace.Span) {
	return startAckSpan(ctx, "mailack.Apply",
		attribute.String("ack.job_id", jobID),
	)
}

func startJobSpan(ctx context.Context) (context.Context, trace.Span) {
	return startAckSpan(ctx, "mailack.StartJob")
}

func startFlagAddSpan(ctx context.Context, jobID, objectHash, folder, envelopeID string) (context.Context, trace.Span) {
	return startAckSpan(ctx, "mailack.FlagAdd",
		attribute.String("ack.job_id", jobID),
		attribute.String("ack.object_hash", objectHash),
		attribute.String("ack.folder", folder),
		attribute.String("ack.envelope_id", envelopeID),
		attribute.String("ack.flag", "seen"),
	)
}

func setAckJobID(span trace.Span, jobID string) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.String("ack.job_id", jobID))
}

func setAckCounts(span trace.Span, failed, seen int) {
	if span == nil {
		return
	}
	span.SetAttributes(
		attribute.Int("ack.failed", failed),
		attribute.Int("ack.seen", seen),
	)
}

func startAckSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailack")
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func endAckSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
