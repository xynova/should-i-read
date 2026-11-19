package mailjob

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func startApplySpan(ctx context.Context, spec Spec, jobID string) (context.Context, trace.Span) {
	return startJobSpan(ctx, spec.SpanApply,
		attribute.String(spec.AttrPrefix+".job_id", jobID),
		attribute.String("mailjob.kind", string(spec.Kind)),
	)
}

func startHopSpan(ctx context.Context, spec Spec, jobID, objectHash, folder, envelopeID string) (context.Context, trace.Span) {
	p := spec.AttrPrefix
	return startJobSpan(ctx, spec.SpanHop,
		attribute.String(p+".job_id", jobID),
		attribute.String(p+".object_hash", objectHash),
		attribute.String(p+".folder", folder),
		attribute.String(p+".envelope_id", envelopeID),
		attribute.String("mailjob.kind", string(spec.Kind)),
	)
}

func startApplyAsyncSpan(ctx context.Context, spec Spec, jobID string) (context.Context, trace.Span) {
	return startJobSpan(ctx, "mailjob.ApplyAsync",
		attribute.String(spec.AttrPrefix+".job_id", jobID),
		attribute.String("mailjob.kind", string(spec.Kind)),
	)
}

func setApplyCounts(span trace.Span, spec Spec, failed, success int) {
	if span == nil {
		return
	}
	span.SetAttributes(
		attribute.Int(spec.AttrPrefix+".failed", failed),
		attribute.Int(spec.AttrPrefix+"."+spec.CountAttr, success),
	)
}

func startJobSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.Tracer("github.com/xynova/should-i-read/internal/mailjob")
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func endJobSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
