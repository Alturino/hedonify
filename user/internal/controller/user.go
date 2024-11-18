package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common/response"
	"github.com/Alturino/ecommerce/internal/log"
	inErrors "github.com/Alturino/ecommerce/user/internal/common/errors"
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
	router.HandleFunc("/login", controller.Login).Methods("POST")
	router.HandleFunc("/register", controller.Register).Methods("POST")
}

func (u UserController) Login(w http.ResponseWriter, r *http.Request) {
	c, span := inOtel.Tracer.Start(r.Context(), "UserController Login")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "UserController Login").
		Logger()
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("decoding request body")
	reqBody := request.LoginRequest{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%s", err.Error())
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "validating requestbody").
			Msg(err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().
		Any(log.KeyRequestBody, reqBody).
		Logger()
	c = logger.WithContext(c)
	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("decoded request body")

	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("initialized validator")

	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil {
		err = fmt.Errorf("failed validating request body with error=%s", err.Error())
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "validating requestbody").
			Msg(err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("validated request body")

	logger.Info().
		Str(log.KeyProcess, "login").
		Msg("login")
	token, err := u.service.Login(c, reqBody)
	if err != nil && errors.Is(err, inErrors.ErrUserNotFound) {
		logger.Error().
			Err(errors.Join(err, inErrors.ErrUserNotFound)).
			Str(log.KeyProcess, "login").
			Msgf("failed finding user by email=%s not found", reqBody.Email)
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    inErrors.ErrUserNotFound.Error(),
		})
		return
	}
	logger.Info().
		Str(log.KeyProcess, "login").
		Msg("login success")

	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("decoding request body")
	reqBody := request.RegisterRequest{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "validating requestbody").
			Msgf("failed decoding request body with error=%s", err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().
		Any(log.KeyRequestBody, reqBody).
		Logger()
	c = logger.WithContext(r.Context())
	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("decoded request body")

	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("initialized validator")

	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil &&
		errors.Is(err, &validator.InvalidValidationError{}) {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "validating requestbody").
			Msgf("failed validating request body with error=%s", err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().
		Str(log.KeyProcess, "validating requestbody").
		Msg("validated request body")

	logger.Info().
		Str(log.KeyProcess, "registering user").
		Msg("registering user")
	user, err := u.service.Register(c, reqBody)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "registering user").
			Msgf("failed registering user with error=%s", err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().
		Str(log.KeyProcess, "registering user").
		Msg("registered user")

	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message": fmt.Sprintf(
			"user with username=%s and email=%s is registered",
			user.Username,
			user.Email,
		),
	})
}
