package middleware

import (
	"net/http"

	"github.com/rs/zerolog"

	inHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/otel"
)

func RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, span := otel.Tracer.Start(r.Context(), "RecoverPanic")
		defer span.End()

		logger := zerolog.Ctx(c).With().Logger()
		defer func() {
			if err := recover(); err != nil {
				logger.Error().Err(err.(error)).Stack().Msg("recovered from panic")
				otel.RecordError(err.(error), span)
				inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
					"status":     "failed",
					"statusCode": http.StatusInternalServerError,
					"message":    "Internal Server Error",
				})
				return
			}
		}()

		next.ServeHTTP(w, r)
	})
}
