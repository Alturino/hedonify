package internal

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/errors"
	"github.com/Alturino/ecommerce/internal/otel"
)

func VerifyToken(c context.Context, token string) (*jwt.Token, error) {
	c, span := otel.Tracer.Start(c, "VerifyToken")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "VerifyToken").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing config").Logger()
	logger.Trace().Msg("initializing config")
	c = logger.WithContext(c)
	cfg := config.Get(c, constants.APP_USER_SERVICE)
	logger.Trace().Msg("initialized config")

	logger = logger.With().Str(constants.KEY_PROCESS, "parsing claims").Logger()
	logger.Trace().Msg("parsing claims")
	jwtToken, err := jwt.ParseWithClaims(token,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (interface{}, error) {
			return []byte(cfg.SecretKey), nil
		},
		jwt.WithAudience(constants.AUDIENCE_USER),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(constants.APP_USER_SERVICE),
	)
	if err != nil {
		err = fmt.Errorf("failed parsing claims with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		otel.RecordError(err, span)
		return nil, err
	}
	logger = logger.With().Any(constants.KEY_TOKEN, jwtToken).Logger()
	logger.Info().Msg("parsed claims")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating token").Logger()
	logger.Trace().Msg("validating token")
	if !jwtToken.Valid {
		err = fmt.Errorf("failed validating token with error=%w", errors.ErrTokenInvalid)
		otel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return nil, errors.ErrTokenInvalid
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

func UserIdFromJwtToken(c context.Context) (uuid.UUID, error) {
	c, span := otel.Tracer.Start(c, "UserIdFromJwtToken")
	defer span.End()

	logger := zerolog.Ctx(c).With().Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "getting userId from jwtToken").Logger()
	logger.Trace().Msg("getting jwtToken from context")
	span.AddEvent("getting jwtToken from context")
	jwt := JwtTokenFromContext(c)
	subject, err := jwt.Claims.GetSubject()
	if err != nil {
		err = fmt.Errorf("failed getting subject from jwt with error=%w", err)
		otel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return uuid.Nil, err
	}
	span.AddEvent("got subject from jwtToken")
	logger.Info().Msg("got subject from jwtToken")

	logger.Trace().Msg("parsing subject")
	span.AddEvent("parsing subject")
	userId, err := uuid.Parse(subject)
	if err != nil {
		err = fmt.Errorf("failed parsing subject=%s with error=%w", subject, err)
		otel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return uuid.Nil, err
	}
	span.AddEvent("parsed subject as userId")
	logger = logger.With().Str(constants.KEY_USER_ID, userId.String()).Logger()
	logger.Info().Msg("parsed subject as userId")

	return userId, nil
}
