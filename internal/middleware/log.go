package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/internal/common/otel"
	"github.com/Alturino/ecommerce/internal/log"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(log.KeyRequestID)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c, span := otel.Tracer.Start(
			r.Context(),
			"main Logging",
			trace.WithAttributes(
				attribute.String(log.KeyRequestID, requestID),
				attribute.String(log.KeyRequestHost, r.Host),
				attribute.String(log.KeyRequestIp, r.RemoteAddr),
				attribute.String(log.KeyRequestMethod, r.Method),
				attribute.String(log.KeyRequestURI, r.RequestURI),
				attribute.String(log.KeyRequestURL, r.URL.String()),
			),
		)
		defer span.End()

		var buffer bytes.Buffer
		teeReader := io.TeeReader(r.Body, &buffer)
		requestBody := map[string]interface{}{}
		json.NewDecoder(teeReader).Decode(&requestBody)
		r.Body = io.NopCloser(&buffer)

		logger := zerolog.Ctx(c).
			With().
			Str(log.KeyRequestID, requestID).
			Dict(log.KeyRequest, zerolog.Dict().
				Any(log.KeyRequestHeader, r.Header).
				Str(log.KeyRequestHost, r.Host).
				Str(log.KeyRequestIp, r.RemoteAddr).
				Str(log.KeyRequestMethod, r.Method).
				Str(log.KeyRequestURI, r.RequestURI).
				Str(log.KeyRequestURL, r.URL.String()).
				Any(log.KeyRequestBody, requestBody)).
			Str(log.KeyTag, "Logging").
			Logger()
		logger.Info().Msg("attached request value to logger")

		logger.Info().Msg("attaching request value to context")
		c = log.AttachRequestIDToContext(c, requestID)
		c = logger.WithContext(c)
		r = r.WithContext(c)
		logger.Info().Msg("attached request value to context")

		logger.Info().Msg("next handler")
		next.ServeHTTP(w, r)
	})
}
