package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/internal/log"
	inTrace "github.com/Alturino/ecommerce/internal/otel/trace"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()
		c, span := inTrace.Tracer.Start(
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

		logger := zerolog.Ctx(c).
			With().
			Str(log.KeyRequestID, requestID).
			Any(log.KeyRequestHeader, r.Header).
			Str(log.KeyRequestHost, r.Host).
			Str(log.KeyRequestIp, r.RemoteAddr).
			Str(log.KeyRequestMethod, r.Method).
			Str(log.KeyRequestURI, r.RequestURI).
			Str(log.KeyRequestURL, r.URL.String()).
			Str(log.KeyTag, "Logging").
			Logger()
		logger.Info().Msg("attached request value to logger")

		logger.Info().Msg("attaching request value to context")
		c = log.AttachRequestIDToContext(c, requestID)
		c = logger.WithContext(c)
		newR := r.WithContext(c)
		logger.Info().Msg("attached request value to context")

		logger.Info().Msg("next handler")
		next.ServeHTTP(w, newR)
	})
}
