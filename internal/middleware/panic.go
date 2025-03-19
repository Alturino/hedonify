package middleware

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	inHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/otel"
)

func RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, span := otel.Tracer.Start(r.Context(), "RecoverPanic")
		defer span.End()

		logger := zerolog.Ctx(c).With().Ctx(c).Logger()
		defer func() {
			if r := recover(); r != nil {
				err := errors.Cause(r.(error))
				logger.Error().Err(err).Stack().Msg("recovered from panic")
				otel.RecordError(err, span)
				inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
					"status":     "failed",
					"statusCode": http.StatusInternalServerError,
					"message":    "Internal Server Error",
				})
				return
			}
		}()

		r = r.WithContext(logger.WithContext(c))
		next.ServeHTTP(w, r)
	})
}
