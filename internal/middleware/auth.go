package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal"
	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/errors"
	commonHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/otel"
)

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, span := otel.Tracer.Start(r.Context(), "Auth")
		defer span.End()

		authorization := r.Header.Get("Authorization")

		logger := zerolog.Ctx(c).
			With().
			Str(constants.KEY_TAG, "middleware Auth").
			Logger()

		if authorization == "" {
			err := fmt.Errorf(
				"failed checking authorization header with error=%w",
				errors.ErrEmptyAuth,
			)
			otel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
				"status":     "failed",
				"statusCode": http.StatusUnauthorized,
				"message":    errors.ErrEmptyAuth.Error(),
			})
			return
		}
		logger.Info().Msg("authorization header checked")

		logger = logger.With().Str(constants.KEY_PROCESS, "verifying token").Logger()
		logger.Info().Msg("verifying token")
		token := strings.Split(authorization, " ")[1]
		c = logger.WithContext(c)
		jwt, err := internal.VerifyToken(c, token)
		if err != nil {
			err = fmt.Errorf("failed verifying token with error=%w", err)
			otel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
				"status":     "failed",
				"statusCode": http.StatusUnauthorized,
				"message":    errors.ErrTokenInvalid.Error(),
			})
			return
		}
		logger.Info().Msg("verified token")

		logger = logger.With().Str(constants.KEY_PROCESS, "attaching jwt token to context").Logger()
		logger.Info().Msg("attaching jwt token to context")
		c = internal.AttachJwtToken(c, jwt)
		c = logger.WithContext(c)
		r = r.WithContext(c)
		logger.Info().Msg("attached jwt token to context")

		next.ServeHTTP(w, r)
	})
}
