package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/log"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()

		pLogger := zerolog.Ctx(r.Context())
		logger := pLogger.With().
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
		c := log.AttachRequestIDToContext(r.Context(), requestID)
		c = logger.WithContext(c)
		newR := r.WithContext(c)
		logger.Info().Msg("attached request value to context")

		logger.Info().Msg("next handler")
		next.ServeHTTP(w, newR)
	})
}
