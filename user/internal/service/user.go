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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/user/internal/common/cache"
	userErrors "github.com/Alturino/ecommerce/user/internal/common/errors"
	userOtel "github.com/Alturino/ecommerce/user/internal/common/otel"
	"github.com/Alturino/ecommerce/user/pkg/request"
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

	cacheKey := fmt.Sprintf(cache.LOGIN_USER, param.Email)
	logger := zerolog.Ctx(c).
		With().
		Str(log.KEY_TAG, "UserService Login").
		Str(log.KEY_EMAIL, param.Email).
		Str(log.KEY_CACHE_KEY, cacheKey).
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "getting token from cache").Logger()
	logger.Info().Msg("getting token from cache")
	span.AddEvent("getting token from cache")
	signedToken, err := u.cache.JSONGet(c, cacheKey).Result()
	if (err != nil || errors.Is(err, redis.Nil)) || signedToken == "" {
		err = fmt.Errorf("failed getting token from cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())
		span.AddEvent("failed getting token from cache")

		logger = logger.With().Str(log.KEY_PROCESS, "finding user by email").Logger()
		logger.Info().Msg("finding user by email")
		span.AddEvent("finding user by email")
		user, err := u.queries.FindByEmail(c, param.Email)
		if err != nil {
			err = errors.Join(err, userErrors.ErrUserNotFound)
			err = fmt.Errorf("failed finding user by email=%s with error=%w", param.Email, err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return "", err
		}
		span.AddEvent("found user by email")
		logger.Info().Msg("found user by email")

		logger = logger.With().
			Str(log.KEY_PROCESS, "verifying hashed password with password").
			Logger()
		logger.Info().Msg("verifying hashed password with password")
		span.AddEvent("verifying hashed password with password")
		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(param.Password))
		if err != nil {
			err = errors.Join(err, userErrors.ErrPasswordMismatch)
			err = fmt.Errorf("failed verifying hashed password and password with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return "", err
		}
		logger.Info().Msg("verified hashed password with password")
		span.AddEvent("verified hashed password with password")

		logger = logger.With().Str(log.KEY_PROCESS, "creating login token").Logger()
		span.AddEvent("creating login token")
		logger.Info().Msg("creating login token")
		tokenCreationTime := time.Now()
		token := jwt.NewWithClaims(
			jwt.SigningMethodHS256,
			jwt.RegisteredClaims{
				Audience:  jwt.ClaimStrings{constants.AUDIENCE_USER},
				Issuer:    constants.APP_USER_SERVICE,
				Subject:   user.ID.String(),
				ExpiresAt: jwt.NewNumericDate(tokenCreationTime.Add(30 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(tokenCreationTime),
				ID:        uuid.NewString(),
			},
		)
		logger.Info().Msg("created login token")
		span.AddEvent("created login token")

		logger = logger.With().Str(log.KEY_PROCESS, "signing token").Logger()
		logger.Info().Msg("signing token")
		span.AddEvent("signing token")
		signedToken, err = token.SignedString([]byte(u.config.SecretKey))
		if err != nil {
			err = fmt.Errorf("failed signing token with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return "", err
		}
		logger = logger.With().Str(log.KEY_TOKEN, signedToken).Logger()
		span.AddEvent("signed token")
		logger.Info().Msg("signed token")

		logger = logger.With().Str(log.KEY_PROCESS, "inserting token to cache").Logger()
		logger.Info().Msg("inserting token to cache")
		span.AddEvent("inserting token to cache")
		err = u.cache.SetEx(c, cacheKey, signedToken, 25*time.Minute).Err()
		if err != nil {
			err = fmt.Errorf("failed inserting token to cache with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return "", err
		}
		logger.Info().Msg("inserted token to cache")
		span.AddEvent("inserted token to cache")
	}
	logger.Info().RawJSON(log.KEY_JSON_CACHE, []byte(signedToken)).Msg("got token from cache")
	span.AddEvent(
		"got token from cache",
		trace.WithAttributes(attribute.String(log.KEY_JSON_CACHE, signedToken)),
	)

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
		Str(log.KEY_TAG, "UserService Register").
		Str(log.KEY_EMAIL, param.Email).
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "checking if email exists").Logger()
	logger.Info().Msg("checking if email exists")
	span.AddEvent("checking if email exists")
	_, err := svc.queries.FindByEmail(c, param.Email)
	if err == nil {
		err = userErrors.ErrEmailExist
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.User{}, err
	}
	span.AddEvent("checked email not exist")
	logger.Info().Msg("checked email not exist")

	logger = logger.With().Str(log.KEY_PROCESS, "hashing password").Logger()
	span.AddEvent("hashing password")
	logger.Info().Msg("hashing password")
	hashed, err := bcrypt.GenerateFromPassword([]byte(param.Password), bcrypt.DefaultCost)
	if err != nil {
		err = errors.Join(err, commonErrors.ErrFailedHashToken)
		err = fmt.Errorf("failed hashing password with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.User{}, err
	}
	span.AddEvent("hashed password")
	logger.Info().Msg("hashed password")

	logger = logger.With().Str(log.KEY_PROCESS, "inserting user to database").Logger()
	logger.Info().Msg("inserting user to database")
	span.AddEvent("inserting user to database")
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
	span.AddEvent("inserted user to database")

	logger = logger.With().Str(log.KEY_PROCESS, "inserting user to cache").Logger()
	span.AddEvent("inserting user to cache")
	logger.Info().Msg("inserting user to cache")
	err = svc.cache.JSONSet(c, fmt.Sprintf(cache.KEY_USER, user.ID.String()), "$", user).Err()
	if err != nil {
		err = fmt.Errorf("failed inserting user to cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return user, nil
	}
	logger.Info().Msg("inserted user to cache")
	span.AddEvent("inserted user to cache")

	return user, nil
}

func (svc UserService) FindUserById(
	c context.Context,
	param request.FindUserById,
) (user repository.User, err error) {
	c, span := userOtel.Tracer.Start(c, "UserService FindUserById")
	defer span.End()

	cacheKey := fmt.Sprintf(cache.KEY_USER, param.ID.String())

	logger := zerolog.Ctx(c).
		With().
		Str(log.KEY_TAG, "UserService FindUserById").
		Str(log.KEY_CACHE_KEY, cacheKey).
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "finding user by id in cache").Logger()
	logger.Info().Msg("finding user by id in cache")
	jsonCache, err := svc.cache.JSONGet(c, cacheKey).Result()
	if (err != nil || errors.Is(err, redis.Nil)) || jsonCache == "" {
		err = fmt.Errorf("failed finding user by id from cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		logger = logger.With().Str(log.KEY_PROCESS, "finding user by id in database").Logger()
		logger.Info().Msg("finding user by id in database")
		user, err = svc.queries.FindById(c, param.ID)
		if err != nil {
			err = fmt.Errorf("failed finding user from database with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return repository.User{}, err
		}
		logger = logger.With().Any(log.KEY_USER, user).Logger()
		logger.Info().Msg("found user in database")

		logger.With().Str(log.KEY_PROCESS, "cache user").Logger()
		logger.Info().Msg("inserting user to cache")
		span.AddEvent("inserting user to cache")
		err = svc.cache.JSONSet(c, cacheKey, "$", user).Err()
		if err != nil {
			err = fmt.Errorf("failed inserting user to cache with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return repository.User{}, err
		}
		logger.Info().Msg("inserted user to cache")
		span.AddEvent("inserted user to cache")

		return user, err
	}
	logger = logger.With().RawJSON(log.KEY_JSON_CACHE, []byte(jsonCache)).Logger()
	logger.Info().Msg("found user by id in cache")

	logger = logger.With().Str(log.KEY_PROCESS, "unmarshaling user from cache").Logger()
	logger.Info().Msg("unmarshaling user from cache")
	err = json.Unmarshal([]byte(jsonCache), &user)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling user from cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.User{}, err
	}
	logger = logger.With().Any(log.KEY_USER, user).Logger()
	logger.Info().Msg("unmarshaled user from cache")

	return user, nil
}
