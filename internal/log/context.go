package log

import (
	"context"

	"github.com/rs/zerolog"
	zl "github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/internal/constants"
)

type requestId struct{}

func RequestIDFromContext(c context.Context) string {
	return c.Value(requestId{}).(string)
}

func AttachRequestIDToContext(c context.Context, h string) context.Context {
	return context.WithValue(c, requestId{}, h)
}

func AttachTraceIdFromContext() zerolog.HookFunc {
	return func(e *zerolog.Event, level zerolog.Level, message string) {
		c := e.GetCtx()
		spanCtx := trace.SpanContextFromContext(e.GetCtx())

		reqId := RequestIDFromContext(c)
		traceId := spanCtx.TraceID().String()
		spanId := spanCtx.SpanID().String()
		logger := zl.With().
			Str(constants.KEY_TAG, "log AttachTraceIdFromContext").
			Str(constants.KEY_REQUEST_ID, reqId).
			Str(constants.KEY_TRACE_ID, traceId).
			Str(constants.KEY_SPAN_ID, spanId).
			Logger()

		e.Str(constants.KEY_REQUEST_ID, reqId)
		if spanCtx.IsValid() {
			e.Str(constants.KEY_TRACE_ID, traceId).Str(constants.KEY_SPAN_ID, spanId)
			logger.Debug().Msg("span context is valid")
		}
	}
}
