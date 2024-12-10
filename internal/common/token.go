package common

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	inErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
)

func VerifyToken(c context.Context, token string) error {
	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "VerifyToken").
		Str(log.KeyAuthToken, token).
		Logger()

	claims := jwt.RegisteredClaims{}

	logger = logger.With().Str(log.KeyProcess, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	config := config.InitConfig(c, AppUserService)
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "parsing claims").Logger()
	jwtToken, err := jwt.ParseWithClaims(token,
		claims,
		func(t *jwt.Token) (interface{}, error) {
			return config.Application.SecretKey, nil
		},
		jwt.WithAudience(AudienceUser),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(AppUserService),
	)
	if err != nil {
		err = fmt.Errorf("failed parsing with claims with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	logger = logger.With().Any(log.KeyToken, jwtToken).Logger()
	logger.Info().Msg("parsed claims")

	logger = logger.With().Str(log.KeyProcess, "validating token").Logger()
	logger.Info().Msg("validating token")
	if !jwtToken.Valid {
		logger.Error().Err(inErrors.ErrTokenInvalid).Msg(inErrors.ErrTokenInvalid.Error())
		return inErrors.ErrTokenInvalid
	}
	logger.Info().Msg("validated token")

	return nil
}
