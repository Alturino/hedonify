package controller

import (
	"encoding/json"
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
}

func (t *CartController) InsertCart(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController InsertCart")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartController InsertCart").
		Logger()
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "decoding requestbody").
		Msg("decoding requestbody")
	reqBody := request.InsertCartRequest{}
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
	logger = logger.With().
		Any(log.KeyRequestBody, reqBody).
		Logger()
	c = logger.WithContext(c)
	logger.Info().
		Str(log.KeyProcess, "decoding requestbody").
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
		Str(log.KeyProcess, "inserting cart").
		Msg("inserting cart")
	cart, err := t.service.InsertCart(c, reqBody)
	if err != nil {
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
		Str(log.KeyProcess, "inserting cart").
		Msg("inserted cart")
	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully inserted cart",
		"data": map[string]interface{}{
			"cart": cart,
		},
	})
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

	logger := zerolog.Ctx(c).With().Str(log.KeyTag, "CartController FindCarts").Logger()

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
