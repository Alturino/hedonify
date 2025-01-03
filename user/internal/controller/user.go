package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	commonHttp "github.com/Alturino/ecommerce/internal/common/http"
	"github.com/Alturino/ecommerce/internal/log"
	userErrors "github.com/Alturino/ecommerce/user/internal/common/errors"
	inOtel "github.com/Alturino/ecommerce/user/internal/common/otel"
	"github.com/Alturino/ecommerce/user/internal/request"
	"github.com/Alturino/ecommerce/user/internal/service"
)

type UserController struct {
	service *service.UserService
}

func AttachUserController(c context.Context, mux *mux.Router, service *service.UserService) {
	router := mux.PathPrefix("/users").Subrouter()

	controller := UserController{service: service}
	router.HandleFunc("/login", controller.Login).Methods(http.MethodPost)
	router.HandleFunc("/register", controller.Register).Methods(http.MethodPost)
	router.HandleFunc("/{userId}", controller.FindUserById).Methods(http.MethodGet)
}

func (u UserController) Login(w http.ResponseWriter, r *http.Request) {
	c, span := inOtel.Tracer.Start(r.Context(), "UserController Login")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "UserController Login").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "decoding requestbody").Logger()
	logger.Info().Msg("decoding request body")
	reqBody := request.LoginRequest{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%s", err.Error())

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyRequestBody, reqBody).Logger()
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(log.KeyProcess, "validating requestbody").Logger()
	logger.Info().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().Msg("initialized validator")

	logger.Info().Msg("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil {
		err = fmt.Errorf("failed validating request body with error=%s", err.Error())

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(log.KeyProcess, "login").Logger()
	logger.Info().Msg("login")
	token, err := u.service.Login(c, reqBody)
	if err != nil && errors.Is(err, userErrors.ErrUserNotFound) {
		err = errors.Join(err, userErrors.ErrUserNotFound)
		err = fmt.Errorf("failed login with error=%w", err)

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    userErrors.ErrUserNotFound.Error(),
		})
		return
	}
	logger.Info().Msg("login success")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "login success",
		"data": map[string]string{
			"token": token,
		},
	})
}

func (u UserController) Register(w http.ResponseWriter, r *http.Request) {
	c, span := inOtel.Tracer.Start(r.Context(), "UserController Register")
	defer span.End()

	logger := zerolog.Ctx(r.Context()).
		With().
		Str(log.KeyTag, "UserController Register").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "decoding request body").Logger()
	logger.Info().Msg("decoding request body")
	reqBody := request.Register{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "validating requestbody").
			Msgf("failed decoding request body with error=%s", err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyRequestBody, reqBody).Logger()
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(log.KeyProcess, "validating request body").Logger()
	logger.Info().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().Msg("initialized validator")

	logger.Info().Msg("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil &&
		errors.Is(err, &validator.InvalidValidationError{}) {
		err = fmt.Errorf("failed validating request body with error=%w", err)

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(log.KeyProcess, "registering user").Logger()
	logger.Info().Msg("registering user")
	user, err := u.service.Register(c, reqBody)
	if err != nil {
		err = fmt.Errorf("failed registering user with error=%w", err)

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("registered user")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message": fmt.Sprintf(
			"user with username=%s and email=%s is registered",
			user.Username,
			user.Email,
		),
	})
}

func (u UserController) FindUserById(w http.ResponseWriter, r *http.Request) {
	c, span := inOtel.Tracer.Start(r.Context(), "UserController FindUserById")
	defer span.End()

	logger := zerolog.Ctx(r.Context()).
		With().
		Str(log.KeyTag, "UserController Register").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "get userId from path").Logger()
	logger.Info().Msg("get userId from path")
	userId, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		err = fmt.Errorf("failed parsing userId with error=%w", err)

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
	}
	logger = logger.With().Str(log.KeyUserID, userId.String()).Logger()
	logger.Info().Msg("parsed userId from path")

	logger = logger.With().Str(log.KeyProcess, "finding user by id").Logger()
	logger.Info().Msg("finding user by id")
	c = logger.WithContext(c)
	user, err := u.service.FindUserById(c, request.FindUserById{ID: userId})
	if err != nil {
		err = fmt.Errorf("failed finding user by id with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
	}
	logger = logger.With().Any(log.KeyUser, user).Logger()
	logger.Info().Msg("found user by id")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("user with id=%s is found", user.ID.String()),
	})
}
