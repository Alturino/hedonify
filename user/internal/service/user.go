package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/Alturino/ecommerce/internal/common"
	globalErr "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
	inErrors "github.com/Alturino/ecommerce/user/internal/common/errors"
	inOtel "github.com/Alturino/ecommerce/user/internal/common/otel"
	"github.com/Alturino/ecommerce/user/internal/repository"
	"github.com/Alturino/ecommerce/user/internal/request"
)

type UserService struct {
	queries *repository.Queries
	config  config.Application
}

func NewUserService(queries *repository.Queries, config config.Application) *UserService {
	return &UserService{queries: queries, config: config}
}

func (u *UserService) Login(
	c context.Context,
	param request.LoginRequest,
) (string, error) {
	c, span := inOtel.Tracer.Start(c, "UserService Login")
	defer span.End()

	logger := zerolog.Ctx(c).With().
		Str(log.KeyTag, "UserService Login").
		Str(log.KeyEmail, param.Email).
		Logger()

	logger.Info().
		Str(log.KeyProcess, "finding user").
		Str(log.KeyEmail, param.Email).
		Msg("finding user by email")
	user, err := u.queries.FindByEmail(c, param.Email)
	if err != nil {
		logger.Error().
			Err(inErrors.ErrUserNotFound).
			Str(log.KeyProcess, "finding user").
			Msgf("failed finding user by email=%s not found", param.Email)
		return "", errors.Join(err, inErrors.ErrUserNotFound)
	}
	logger.Info().
		Str(log.KeyProcess, "finding user").
		Msg("found user by email")

	logger.Info().
		Str(log.KeyProcess, "verifying password").
		Msg("Verifying hashed password with password")
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(param.Password))
	if err != nil {
		logger.Error().
			Err(inErrors.ErrPasswordMismatch).
			Str(log.KeyProcess, "verifying password").
			Msg("Failed verifying hashed password and password is mismatch")
		return "", inErrors.ErrPasswordMismatch
	}
	logger.Info().
		Str(log.KeyProcess, "verifying password").
		Msg("Verified hashed password with password")

	logger.Info().
		Str(log.KeyProcess, "creating login token").
		Msg("creating login token")
	tokenCreationTime := time.Now()
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{common.AudienceUser},
			Issuer:    common.AppUserService,
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(tokenCreationTime.Add(30 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(tokenCreationTime),
		},
	)
	logger.Info().
		Str(log.KeyProcess, "creating login token").
		Msg("created login token")

	logger.Info().
		Str(log.KeyProcess, "signing token").
		Msg("signing token")
	signedToken, err := token.SignedString([]byte(u.config.SecretKey))
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "verifying password").
			Msgf("Failed signing token with error=%s", err.Error())
		return "", err
	}
	logger.Info().
		Str(log.KeyProcess, "signing token").
		Msg("signed token")

	return signedToken, nil
}

func (u *UserService) Register(
	c context.Context,
	param request.RegisterRequest,
) (repository.User, error) {
	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "UserService Register").
		Str(log.KeyEmail, param.Email).
		Logger()

	logger.Info().
		Str(log.KeyProcess, "hashing password").
		Msg("hashing password")
	hashed, err := bcrypt.GenerateFromPassword([]byte(param.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "hashing password").
			Msgf("failed hashing password with error=%s", err.Error())
		return repository.User{}, errors.Join(err, globalErr.ErrFailedHashToken)
	}
	logger.Info().
		Str(log.KeyProcess, "hashing password").
		Msg("hashed password")

	logger.Info().
		Str(log.KeyProcess, "inserting user to database").
		Msg("inserting user to database")
	user, err := u.queries.InsertUser(c, repository.InsertUserParams{
		Username:  param.Username,
		Email:     param.Email,
		Password:  string(hashed),
		CreatedAt: pgtype.Timestamp{Time: time.Now()},
		UpdatedAt: pgtype.Timestamp{Time: time.Now()},
	})
	if err != nil {
		logger.Error().
			Err(err).
			Msgf("failed inserting user to database with error=%s", err.Error())
		return repository.User{}, err
	}
	logger.Info().
		Str(log.KeyProcess, "inserting user to database").
		Msg("inserted user to database")

	return user, nil
}
