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
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	"github.com/Alturino/ecommerce/internal/constants"
	inHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/middleware"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	userErrors "github.com/Alturino/ecommerce/user/internal/errors"
	"github.com/Alturino/ecommerce/user/internal/otel"
	"github.com/Alturino/ecommerce/user/internal/service"
	"github.com/Alturino/ecommerce/user/pkg/request"
)

type UserController struct {
	service *service.UserService
}

func AttachUserController(c context.Context, mux *mux.Router, service *service.UserService) {
	controller := UserController{service: service}

	router := mux.PathPrefix("/users").Subrouter()
	router.Use(
		otelmux.Middleware(constants.APP_USER_SERVICE),
		middleware.Logging,
		middleware.RecoverPanic,
	)
	router.HandleFunc("/login", controller.Login).Methods(http.MethodPost)
	router.HandleFunc("/register", controller.Register).Methods(http.MethodPost)
	router.HandleFunc("/{userId}", controller.FindUserById).Methods(http.MethodGet)
}

func (u UserController) Login(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "UserController Login")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "UserController Login").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "decoding requestbody").Logger()
	logger.Trace().Msg("decoding request body")
	span.AddEvent("decoding request body")
	reqBody := request.LoginRequest{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%s", err.Error())
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("decoded request body")
	logger.Trace().Msg("decoded request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating.request_body").Logger()
	logger.Trace().Msg("initializing validator")
	span.AddEvent("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	span.AddEvent("initialized validator")
	logger.Trace().Msg("initialized validator")

	logger.Trace().Msg("validating request body")
	span.AddEvent("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil {
		err = fmt.Errorf("failed validating request body with error=%s", err.Error())
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Trace().Msg("validated request body")
	span.AddEvent("validated request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "login").Logger()
	logger.Trace().Msg("trying to login")
	span.AddEvent("trying to login")
	c = logger.WithContext(c)
	token, err := u.service.Login(c, reqBody)
	if (err != nil && errors.Is(err, userErrors.ErrUserNotFound)) || token == "" {
		err = errors.Join(err, userErrors.ErrUserNotFound)
		err = fmt.Errorf("failed login with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    userErrors.ErrUserNotFound.Error(),
		})
		return
	}
	logger.Info().Msg("login success")
	span.AddEvent("login success")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "login success",
		"data": map[string]string{
			"token": token,
		},
	})
}

func (u UserController) Register(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "UserController Register")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "UserController Register").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "decoding request body").Logger()
	logger.Trace().Msg("decoding request body")
	span.AddEvent("decoding request body")
	reqBody := request.Register{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		logger.Error().
			Err(err).
			Str(constants.KEY_PROCESS, "validating requestbody").
			Msgf("failed decoding request body with error=%s", err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("decoded request body")
	logger.Trace().Msg("decoded request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "validate request body").Logger()
	logger.Trace().Msg("initializing validator")
	span.AddEvent("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	span.AddEvent("initialized validator")
	logger.Trace().Msg("initialized validator")

	logger.Trace().Msg("validating request body")
	span.AddEvent("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil &&
		errors.Is(err, &validator.InvalidValidationError{}) {
		err = fmt.Errorf("failed validating request body with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("validated request body")
	logger.Debug().Msg("validated request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "registering user").Logger()
	logger.Trace().Msg("registering user")
	c = logger.WithContext(c)
	user, err := u.service.Register(c, reqBody)
	if err != nil {
		err = fmt.Errorf("failed registering user with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		if errors.Is(err, userErrors.ErrEmailExist) {
			inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
				"status":     "failed",
				"statusCode": http.StatusConflict,
				"message":    userErrors.ErrEmailExist.Error(),
			})
			return
		}
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("registered user")
	logger.Info().Msg("registered user")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
	c, span := otel.Tracer.Start(r.Context(), "UserController FindUserById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "UserController FindUserById").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "parsing path values").Logger()
	logger.Trace().Msg("parsing path values")
	span.AddEvent("parsing path values")
	pathValues := mux.Vars(r)
	span.AddEvent("parsed path values")
	logger = logger.With().Any(constants.KEY_PATH_VALUES, pathValues).Logger()
	logger.Trace().Msg("parsed path values")

	logger = logger.With().Str(constants.KEY_PROCESS, "getting userId in pathValues").Logger()
	logger.Trace().Msg("getting userId in pathValues")
	span.AddEvent("getting userId in pathValues")
	pathValue, ok := pathValues["userId"]
	if !ok {
		err := errors.New("userId not found in pathValues")
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("userId found in pathValues")
	logger = logger.With().Str(constants.KEY_USER_ID, pathValue).Logger()
	logger.Trace().Msg("userId found in pathValues")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating userId").Logger()
	logger.Trace().Msg("validating userId")
	span.AddEvent("validating userId")
	userId, err := uuid.Parse(pathValue)
	if err != nil {
		err = fmt.Errorf("failed validating userId with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("validated userId")
	logger = logger.With().Str(constants.KEY_USER_ID, userId.String()).Logger()
	logger.Trace().Msg("validated userId")

	logger = logger.With().Str(constants.KEY_PROCESS, "finding user by id").Logger()
	logger.Trace().Msg("finding user by id")
	span.AddEvent("finding user by id")
	c = logger.WithContext(c)
	user, err := u.service.FindUserById(c, request.FindUserById{ID: userId})
	if err != nil {
		err = fmt.Errorf("failed finding user by id with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("found user by id")
	logger = logger.With().Any(constants.KEY_USER, user).Logger()
	logger.Info().Msg("found user by id")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("user with id=%s is found", user.ID.String()),
	})
}
