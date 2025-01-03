package middleware

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	commonHttp "github.com/Alturino/ecommerce/internal/common/http"
	commonOtel "github.com/Alturino/ecommerce/internal/common/otel"
	"github.com/Alturino/ecommerce/internal/log"
)

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, span := commonOtel.Tracer.Start(r.Context(), "Auth")
		defer span.End()

		logger := zerolog.Ctx(c).With().Str(log.KeyTag, "middleware Auth").Logger()

		logger.Info().Msg("checking authorization header")
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			err := fmt.Errorf(
				"failed checking authorization header with error=%w",
				commonErrors.ErrEmptyAuth,
			)

			commonErrors.HandleError(err, logger, span)
			logger.Error().Err(err).Msg(err.Error())

			commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
				"status":     "failed",
				"statusCode": http.StatusUnauthorized,
				"message":    commonErrors.ErrEmptyAuth.Error(),
			})
			return
		}
		logger = logger.With().Str(log.KeyAuthToken, authorization).Logger()
		logger.Info().Msg("authorization header checked")

		logger = logger.With().Str(log.KeyProcess, "verifying token").Logger()
		logger.Info().Msg("verifying token")
		token := authorization[len("bearer "):]
		c = logger.WithContext(c)
		jwt, err := common.VerifyToken(c, token)
		if err != nil {
			err = fmt.Errorf("failed verifying token with error=%w", err)
			commonErrors.HandleError(err, logger, span)
			logger.Error().Err(err).Msg(err.Error())

			commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
				"status":     "failed",
				"statusCode": http.StatusUnauthorized,
				"message":    commonErrors.ErrTokenInvalid.Error(),
			})
			return
		}
		logger.Info().Msg("verified token")

		logger = logger.With().Str(log.KeyProcess, "attaching jwt token to context").Logger()
		logger.Info().Msg("attaching jwt token to context")
		c = common.AttachJwtToken(c, jwt)
		r = r.WithContext(c)
		logger.Info().Msg("attached jwt token to context")

		next.ServeHTTP(w, r)
	})
}
