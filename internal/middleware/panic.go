package middleware

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/otel"
)

func RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, span := otel.Tracer.Start(r.Context(), "RecoverPanic")
		defer span.End()

		logger := zerolog.Ctx(c).With().Logger()
		defer func() {
			if err := recover(); err != nil {
				logger.Error().Err(err.(error)).Msg("recovered from panic")
				panic(err)
			}
			logger.Info().Msg("recovered from panic")
		}()

		next.ServeHTTP(w, r)
	})
}
