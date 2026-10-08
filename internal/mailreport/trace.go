package mailreport

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var mailreportTracer = otel.Tracer("github.com/xynova/should-i-read/internal/mailreport")

func startSeatSpan(ctx context.Context, name string, model string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{}
	if model != "" {
		attrs = append(attrs, attribute.String("model", model))
	}
	return mailreportTracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func endSeatSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
