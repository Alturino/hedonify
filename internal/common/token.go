package common

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/common/otel"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
)

func VerifyToken(c context.Context, token string) (*jwt.Token, error) {
	c, span := otel.Tracer.Start(c, "VerifyToken")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "VerifyToken").
		Str(log.KeyAuthToken, token).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	config := config.InitConfig(c, constants.AppUserService)
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "parsing claims").Logger()
	logger.Info().Msg("parsing claims")
	jwtToken, err := jwt.ParseWithClaims(token,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (interface{}, error) {
			return []byte(config.SecretKey), nil
		},
		jwt.WithAudience(constants.AudienceUser),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(constants.AppUserService),
	)
	if err != nil {
		err = fmt.Errorf("failed parsing claims with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		commonErrors.HandleError(err, logger, span)
		return nil, err
	}
	logger = logger.With().Any(log.KeyToken, jwtToken).Logger()
	logger.Info().Msg("parsed claims")

	logger = logger.With().Str(log.KeyProcess, "validating token").Logger()
	logger.Info().Msg("validating token")
	if !jwtToken.Valid {
		err = fmt.Errorf("failed validating token with error=%w", commonErrors.ErrTokenInvalid)
		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())
		return nil, commonErrors.ErrTokenInvalid
	}
	logger.Info().Msg("validated token")

	return jwtToken, nil
}

type jwtToken struct{}

func AttachJwtToken(c context.Context, jwt *jwt.Token) context.Context {
	return context.WithValue(c, jwtToken{}, jwt)
}

func JwtTokenFromContext(c context.Context) *jwt.Token {
	return c.Value(jwtToken{}).(*jwt.Token)
}
