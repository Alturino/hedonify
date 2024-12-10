package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "UserService Login").
		Str(log.KeyEmail, param.Email).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding user by email").Logger()
	logger.Info().Msg("finding user by email")
	user, err := u.queries.FindByEmail(c, param.Email)
	if err != nil {
		err = errors.Join(err, inErrors.ErrUserNotFound)
		err = fmt.Errorf("failed finding user by email=%s with error=%w", param.Email, err)
		logger.Error().Err(err).Msg(err.Error())
		return "", err
	}
	logger.Info().Msg("found user by email")

	logger = logger.With().Str(log.KeyProcess, "verifying hashed password with password").Logger()
	logger.Info().Msg("verifying hashed password with password")
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(param.Password))
	if err != nil {
		err = errors.Join(err, inErrors.ErrPasswordMismatch)
		err = fmt.Errorf("failed verifying hashed password and password with error=%w", err)
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
			Audience:  jwt.ClaimStrings{common.AudienceUser},
			Issuer:    common.AppUserService,
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
		logger.Error().Err(err).Msg(err.Error())
		return "", err
	}
	logger.Info().Msg("signed token")

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

	logger = logger.With().Str(log.KeyProcess, "hashing password").Logger()
	logger.Info().Msg("hashing password")
	hashed, err := bcrypt.GenerateFromPassword([]byte(param.Password), bcrypt.DefaultCost)
	if err != nil {
		err = errors.Join(err, globalErr.ErrFailedHashToken)
		err = fmt.Errorf("failed hashing password with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.User{}, err
	}
	logger.Info().Msg("hashed password")

	logger = logger.With().Str(log.KeyProcess, "inserting user to database").Logger()
	logger.Info().Msg("inserting user to database")
	user, err := u.queries.InsertUser(c, repository.InsertUserParams{
		Username:  param.Username,
		Email:     param.Email,
		Password:  string(hashed),
		CreatedAt: pgtype.Timestamp{Time: time.Now()},
		UpdatedAt: pgtype.Timestamp{Time: time.Now()},
	})
	if err != nil {
		err = fmt.Errorf("failed inserting user to database with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.User{}, err
	}
	logger.Info().Msg("inserted user to database")

	return user, nil
}
