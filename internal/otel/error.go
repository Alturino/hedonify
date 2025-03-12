package otel

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func RecordError(err error, span trace.Span) {
	if err == nil {
		return
	}
	span.AddEvent(err.Error())
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}
