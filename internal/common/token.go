package common

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common/constants"
	"github.com/Alturino/ecommerce/internal/common/errors"
	inErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/otel/trace"
)

func VerifyToken(c context.Context, token string) error {
	c, span := trace.Tracer.Start(c, "VerifyToken")
	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "VerifyToken").
		Str(log.KeyAuthToken, token).
		Logger()

	claims := jwt.RegisteredClaims{}

	logger = logger.With().Str(log.KeyProcess, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	config := config.InitConfig(c, constants.AppUserService)
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "parsing claims").Logger()
	jwtToken, err := jwt.ParseWithClaims(token,
		claims,
		func(t *jwt.Token) (interface{}, error) {
			return config.SecretKey, nil
		},
		jwt.WithAudience(constants.AudienceUser),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(constants.AppUserService),
	)
	if err != nil {
		err = fmt.Errorf("failed parsing with claims with error=%w", err)
		errors.HandleError(err, logger, span)
		return err
	}
	logger = logger.With().Any(log.KeyToken, jwtToken).Logger()
	logger.Info().Msg("parsed claims")

	logger = logger.With().Str(log.KeyProcess, "validating token").Logger()
	logger.Info().Msg("validating token")
	if !jwtToken.Valid {
		err = fmt.Errorf("failed validating token with error=%w", inErrors.ErrTokenInvalid)
		logger.Error().Err(err).Msg(err.Error())
		return inErrors.ErrTokenInvalid
	}
	logger.Info().Msg("validated token")

	return nil
}
