package http

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/rs/zerolog"
	gootel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/otel"
)

func WriteJsonResponse(
	c context.Context,
	w http.ResponseWriter,
	header map[string]string,
	body map[string]interface{},
) {
	c, span := otel.Tracer.Start(c, "WriteJsonResponse")
	defer span.End()

	reqId := log.RequestIDFromContext(c)
	traceId := span.SpanContext().TraceID().String()
	spanId := span.SpanContext().SpanID().String()
	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_REQUEST_ID, reqId).
		Str(constants.KEY_TRACE_ID, traceId).
		Str(constants.KEY_SPAN_ID, spanId).
		Str(constants.KEY_TAG, "WriteJsonResponse").
		Logger()

	w.Header().Add(KEY_HEADER_CONTENT_TYPE, VALUE_HEADER_APPLICATION_JSON)
	w.Header().Add(KEY_HEADER_REQUEST_ID, reqId)
	gootel.GetTextMapPropagator().Inject(c, propagation.HeaderCarrier(w.Header()))

	logger.Trace().Msg("writing response header")
	var wg sync.WaitGroup
	for k, v := range header {
		wg.Add(1)
		go func() {
			w.Header().Add(k, v)
			wg.Done()
		}()
	}
	wg.Wait()
	logger.Trace().Msg("written response header")

	logger = logger.With().
		Dict(constants.KEY_RESPONSE, zerolog.Dict().Any(constants.KEY_BODY, body).Any(constants.KEY_HEADER, w.Header().Clone())).
		Logger()

	if v, ok := body["statusCode"]; ok {
		w.WriteHeader(v.(int))
	}

	logger.Trace().Msg("writing response")
	err := json.NewEncoder(w).Encode(body)
	if err != nil {
		otel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("written response")
}
