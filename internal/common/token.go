package common

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	inErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
)

func VerifyToken(c context.Context, token string) error {
	logger := zerolog.Ctx(c)

	claims := jwt.RegisteredClaims{}
	config := config.InitConfig(c, AppUserService)

	var subject string
	jwtToken, err := jwt.ParseWithClaims(token,
		claims,
		func(t *jwt.Token) (interface{}, error) {
			sub, err := t.Claims.GetSubject()
			if err != nil {
				logger.Error().
					Err(inErrors.ErrEmptySubject).
					Msg(inErrors.ErrEmptySubject.Error())
				return nil, err
			}
			subject = sub
			return config.Application.SecretKey, nil
		},
		jwt.WithAudience(AudienceUser),
		jwt.WithSubject(subject),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(AppUserService),
	)
	if err != nil {
		logger.Error().Err(err).Msg(err.Error())
		return err
	}

	if !jwtToken.Valid {
		logger.Error().Err(inErrors.ErrTokenInvalid).Msg(inErrors.ErrTokenInvalid.Error())
		return inErrors.ErrTokenInvalid
	}

	return nil
}
