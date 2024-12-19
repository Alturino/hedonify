package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Alturino/ecommerce/cart/internal/common/otel"
	"github.com/Alturino/ecommerce/cart/internal/service"
	"github.com/Alturino/ecommerce/cart/request"
	inHttp "github.com/Alturino/ecommerce/internal/common/http"
	"github.com/Alturino/ecommerce/internal/log"
)

type CartController struct {
	service *service.CartService
}

func AttachCartController(mux *mux.Router, service *service.CartService) {
	controller := CartController{service: service}

	router := mux.PathPrefix("/carts").Subrouter()
	router.HandleFunc("/", controller.InsertCart).Methods("POST")
	router.HandleFunc("/{cartId}", controller.FindCartById).Methods("GET")
	router.HandleFunc("/{cartId}", controller.InsertCartItem).Methods("POST")
	router.HandleFunc("/{cartId}/{cartItemId}", controller.RemoveCartItem).Methods("DELETE")
}

func (t *CartController) InsertCart(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController InsertCart")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartController InsertCart").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "decoding requestbody").Logger()
	logger.Info().Msg("decoding requestbody")
	reqBody := request.Cart{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		err = fmt.Errorf("failed validating request body with error=%w", err)
		logger.Error().Err(err).Str(log.KeyProcess, "validating requestbody").Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(log.KeyProcess, "inserting cart").Logger()
	logger.Info().Msg("inserting cart")
	c = logger.WithContext(c)
	cart, err := t.service.InsertCart(c, reqBody)
	if err != nil {
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("inserted cart")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully inserted cart",
		"data": map[string]interface{}{
			"cart": cart,
		},
	})
}

// TODO: Not Implemented
func (t *CartController) InsertCartItem(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController InsertCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartController InsertCartItem").
		Str(log.KeyProcess, "decoding requestbody").
		Logger()

	logger.Info().Msg("decoding requestbody")
	reqBody := request.InsertCartItem{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyRequestBody, reqBody).Logger()
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(log.KeyProcess, "initializing validator").Logger()
	logger.Info().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().Msg("initialized validator")

	logger = logger.With().Any(log.KeyProcess, "validating request body").Logger()
	logger.Info().Msg("validating request body")
	cart := request.Cart{}
	if err := validate.StructCtx(c, cart); err != nil {
		err = fmt.Errorf("failed validating request body with error=%s", err.Error())
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated request body")

	c = logger.WithContext(c)
	t.service.InsertCartItem(c, request.InsertCartItem{})
}

func (t *CartController) FindCartById(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController FindCartById")
	defer span.End()

	logger := zerolog.Ctx(c).With().
		Str(log.KeyTag, "CartController FindCartById").
		Str(log.KeyProcess, "validating uuid").
		Logger()

	logger.Info().Msg("validating uuid")
	cartId, err := uuid.Parse(r.PathValue("cartId"))
	span.SetAttributes(attribute.String(log.KeyCartID, cartId.String()))
	if err != nil {
		err = fmt.Errorf("failed validating cartId=%s with error=%w", cartId.String(), err)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger = logger.With().Str(log.KeyCartID, cartId.String()).Logger()
	logger.Info().Msgf("validated uuid cartId=%s", cartId.String())

	logger = logger.With().
		Str(log.KeyProcess, fmt.Sprintf("finding cartId=%s", cartId.String())).
		Logger()
	logger.Info().Msgf("finding cartId=%s", cartId.String())
	c = logger.WithContext(c)
	cart, err := t.service.FindCartById(c, cartId)
	if err != nil {
		err = fmt.Errorf("failed finding cartId=%s with error=%w", cartId.String(), err)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyCart, cart).Logger()
	logger.Info().Msgf("found cartId=%s", cartId.String())

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("cartId=%s found", cartId.String()),
		"data": map[string]interface{}{
			"cart": cart,
		},
	})
}

func (t *CartController) RemoveCartItem(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController RemoveCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartController RemoveCartItem").
		Str(log.KeyProcess, "validating cartId").
		Logger()

	logger.Info().Msg("validating cartId is valid uuid")
	cartId, err := uuid.Parse(r.PathValue("cartId"))
	if err != nil {
		err = fmt.Errorf("failed validating cartId=%s with error=%w", cartId.String(), err)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(log.KeyCartID, cartId.String()).Logger()
	logger.Info().Msgf("valid cartId=%s", cartId.String())

	logger = logger.With().Str(log.KeyProcess, "validating cartItemId").Logger()
	logger.Info().Msg("validating cartItemId is valid uuid")
	cartItemId, err := uuid.Parse(r.PathValue("cartItemId"))
	if err != nil {
		err = fmt.Errorf("failed validating cartItemId=%s with error=%w", cartId.String(), err)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(log.KeyCartItemId, cartItemId.String()).Logger()
	logger.Info().Msgf("valid cartItemId=%s", cartItemId.String())

	logger = logger.With().Str(log.KeyProcess, "removing cart item").Logger()
	logger.Info().Msg("removing cart item")
	c = logger.WithContext(c)
	err = t.service.RemoveCartItem(c, request.RemoveCartItem{ID: cartItemId, CartId: cartId})
	if err != nil {
		err = fmt.Errorf(
			"failed removing cartItemId=%s in cartId=%s with error=%w",
			cartItemId.String(),
			cartId.String(),
			err,
		)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusInternalServerError,
			"message":    err.Error(),
		})
		return
	}
}
