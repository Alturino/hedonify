package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/user/internal/common/cache"
	userErrors "github.com/Alturino/ecommerce/user/internal/common/errors"
	userOtel "github.com/Alturino/ecommerce/user/internal/common/otel"
	"github.com/Alturino/ecommerce/user/internal/repository"
	"github.com/Alturino/ecommerce/user/internal/request"
)

type UserService struct {
	config  config.Application
	cache   *redis.Client
	queries *repository.Queries
}

func NewUserService(
	queries *repository.Queries,
	config config.Application,
	cache *redis.Client,
) *UserService {
	return &UserService{queries: queries, config: config, cache: cache}
}

func (u UserService) Login(
	c context.Context,
	param request.LoginRequest,
) (string, error) {
	c, span := userOtel.Tracer.Start(c, "UserService Login")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "UserService Login").
		Str(log.KeyEmail, param.Email).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding user by email").Logger()
	logger.Info().Msg("finding user by email")
	user, err := u.queries.FindByEmail(c, param.Email)
	if err != nil {
		err = errors.Join(err, userErrors.ErrUserNotFound)
		err = fmt.Errorf("failed finding user by email=%s with error=%w", param.Email, err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return "", err
	}
	logger.Info().Msg("found user by email")

	logger = logger.With().Str(log.KeyProcess, "verifying hashed password with password").Logger()
	logger.Info().Msg("verifying hashed password with password")
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(param.Password))
	if err != nil {
		err = errors.Join(err, userErrors.ErrPasswordMismatch)
		err = fmt.Errorf("failed verifying hashed password and password with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return "", err
	}
	logger.Info().Msg("verified hashed password with password")

	logger = logger.With().Str(log.KeyProcess, "creating login token").Logger()
	logger.Info().Msg("creating login token")
	tokenCreationTime := time.Now()
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{constants.AudienceUser},
			Issuer:    constants.AppUserService,
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(tokenCreationTime.Add(30 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(tokenCreationTime),
			ID:        uuid.NewString(),
		},
	)
	logger.Info().Msg("created login token")

	logger = logger.With().Str(log.KeyProcess, "signing token").Logger()
	logger.Info().Msg("signing token")
	signedToken, err := token.SignedString([]byte(u.config.SecretKey))
	if err != nil {
		err = fmt.Errorf("failed signing token with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return "", err
	}
	logger.Info().Msg("signed token")

	return signedToken, nil
}

func (svc UserService) Register(
	c context.Context,
	param request.Register,
) (repository.User, error) {
	c, span := userOtel.Tracer.Start(c, "UserService Register")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "UserService Register").
		Str(log.KeyEmail, param.Email).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "hashing password").Logger()
	logger.Info().Msg("hashing password")
	hashed, err := bcrypt.GenerateFromPassword([]byte(param.Password), bcrypt.DefaultCost)
	if err != nil {
		err = errors.Join(err, commonErrors.ErrFailedHashToken)
		err = fmt.Errorf("failed hashing password with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return repository.User{}, err
	}
	logger.Info().Msg("hashed password")

	logger = logger.With().Str(log.KeyProcess, "inserting user to database").Logger()
	logger.Info().Msg("inserting user to database")
	user, err := svc.queries.InsertUser(c, repository.InsertUserParams{
		Username: param.Username,
		Email:    param.Email,
		Password: string(hashed),
		CreatedAt: pgtype.Timestamptz{
			Time:             time.Now(),
			InfinityModifier: pgtype.Finite,
			Valid:            true,
		},
		UpdatedAt: pgtype.Timestamptz{
			Time:             time.Now(),
			InfinityModifier: pgtype.Finite,
			Valid:            true,
		},
	})
	if err != nil {
		err = fmt.Errorf("failed inserting user to database with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return repository.User{}, err
	}
	logger.Info().Msg("inserted user to database")

	logger = logger.With().Str(log.KeyProcess, "inserting user to cache").Logger()
	logger.Info().Msg("inserting user to cache")
	err = svc.cache.JSONSet(c, fmt.Sprintf(cache.KEY_USER, user.ID.String()), "$", user).Err()
	if err != nil {
		err = fmt.Errorf("failed inserting user to cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return user, nil
	}
	logger.Info().Msg("inserted user to cache")

	return user, nil
}

func (svc UserService) FindUserById(
	c context.Context,
	param request.FindUserById,
) (user repository.User, err error) {
	c, span := userOtel.Tracer.Start(c, "UserService FindUserById")
	defer span.End()

	logger := zerolog.Ctx(c).With().Str(log.KeyTag, "UserService FindUserById").Logger()

	logger = logger.With().Str(log.KeyProcess, "finding user by id in cache").Logger()
	logger.Info().Msg("finding user by id in cache")
	jsonCache, err := svc.cache.JSONGet(c, fmt.Sprintf(cache.KEY_USER, param.ID.String())).Result()
	if err != nil {
		err = fmt.Errorf("failed finding user by id from cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		logger = logger.With().Str(log.KeyProcess, "finding user by id in database").Logger()
		logger.Info().Msg("finding user by id in database")
		user, err = svc.queries.FindById(c, param.ID)
		if err != nil {
			err = fmt.Errorf(
				"failed finding user by id=%s from database with error=%w",
				param.ID.String(),
				err,
			)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return repository.User{}, err
		}
		logger = logger.With().Any(log.KeyUser, user).Logger()
		logger.Info().Msg("found user by id in database")

		return user, err
	}
	logger = logger.With().RawJSON(log.KeyJsonCache, []byte(jsonCache)).Logger()
	logger.Info().Msg("found user by id in cache")

	logger = logger.With().Str(log.KeyProcess, "unmarshaling user from cache").Logger()
	logger.Info().Msg("unmarshaling user from cache")
	err = json.Unmarshal([]byte(jsonCache), &user)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling user from cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.User{}, err
	}
	logger = logger.With().Any(log.KeyUser, user).Logger()
	logger.Info().Msg("unmarshaled user from cache")

	return user, nil
}
