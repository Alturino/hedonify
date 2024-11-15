package middleware

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common"
	inErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/common/response"
	"github.com/Alturino/ecommerce/internal/log"
)

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context()).With().Str(log.KeyTag, "middleware auth").Logger()
		c := logger.WithContext(r.Context())

		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			logger.Error().
				Err(inErrors.ErrEmptyAuth).
				Msg(inErrors.ErrEmptyAuth.Error())
			response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
				"status":     "failed",
				"statusCode": http.StatusUnauthorized,
				"message":    inErrors.ErrEmptyAuth.Error(),
			})
			return
		}

		token := authorization[len("bearer "):]
		err := common.VerifyToken(r.Context(), token)
		if err != nil {
			logger.Error().
				Err(inErrors.ErrEmptySubject).
				Msg(inErrors.ErrEmptySubject.Error())
			response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
				"status":     "failed",
				"statusCode": http.StatusUnauthorized,
				"message":    inErrors.ErrTokenInvalid.Error(),
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
