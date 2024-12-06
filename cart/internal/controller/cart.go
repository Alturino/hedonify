package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/cart/internal/common/otel"
	"github.com/Alturino/ecommerce/cart/internal/service"
	"github.com/Alturino/ecommerce/cart/request"
	"github.com/Alturino/ecommerce/internal/common/response"
	"github.com/Alturino/ecommerce/internal/log"
)

type CartController struct {
	service *service.CartService
}

func AttachCartController(mux *mux.Router, service *service.CartService) {
	controller := CartController{service: service}

	router := mux.PathPrefix("/carts").Subrouter()
	router.HandleFunc("/", controller.InsertCart).Methods("POST")
	router.HandleFunc("/", controller.FindCarts).Methods("GET")
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
		Str(log.KeyProcess, "decoding requestbody").
		Logger()

	logger.Info().Msg("decoding requestbody")
	reqBody := request.InsertCart{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "decoding requestbody").
			Msgf("failed decoding request body with error=%s", err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("inserted cart")
	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
	_, span := otel.Tracer.Start(r.Context(), "CartController InsertCart")
	defer span.End()
	return
}

func (t *CartController) FindCartById(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController FindCartById")
	defer span.End()

	cartId := r.PathValue("cartId")

	logger := zerolog.Ctx(c).With().
		Str(log.KeyTag, "CartController FindCartById").
		Str(log.KeyCartID, cartId).
		Logger()

	logger.Info().
		Str(log.KeyProcess, "validating uuid").
		Msg("checking id is valid uuid")
	uuid, err := uuid.Parse(cartId)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "finding cart").
			Msgf("id=%s is not a valid uuid", cartId)
		return
	}
	logger.Info().
		Str(log.KeyProcess, "validating uuid").
		Msgf("id=%s is not a valid uuid", cartId)

	cart, err := t.service.FindCartById(c, uuid)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "finding cart").
			Msgf("cart by id=%s not found", cartId)
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    "successfully inserted cart",
			"data": map[string]interface{}{
				"cart": cart,
			},
		})
		return
	}
}

func (t *CartController) FindCarts(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController FindCarts")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartController FindCarts").
		Logger()

	logger.Info().
		Str(log.KeyProcess, "get query params").
		Msg("get query params")
	productId, userId := r.URL.Query().Get("productId"), r.URL.Query().Get("userId")
	logger = logger.With().
		Str(log.KeyProcess, "get query params").
		Str(log.KeyProductID, productId).
		Str(log.KeyUserID, userId).
		Logger()

	logger.Info().
		Str(log.KeyProcess, "finding carts").
		Msgf("validating userId=%s is valid uuid", userId)
	userUUID, err := uuid.Parse(userId)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "finding carts").
			Msgf("userId=%s is not valid uuid", userId)
	}
	logger.Info().
		Str(log.KeyProcess, "finding carts").
		Msgf("validated userId=%s is valid uuid", userId)

	logger.Info().
		Str(log.KeyProcess, "finding carts").
		Msg("finding carts")
	carts, err := t.service.FindCartByUserId(c, userUUID)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "finding carts").
			Msg(err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().
		Str(log.KeyProcess, "finding carts").
		Any("carts", carts).
		Msg("found carts")

	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"data": map[string]interface{}{
			"carts": carts,
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
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusInternalServerError,
			"message":    err.Error(),
		})
		return
	}
}
