package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/cart/internal/otel"
	"github.com/Alturino/ecommerce/cart/internal/service"
	"github.com/Alturino/ecommerce/cart/pkg/request"
	"github.com/Alturino/ecommerce/internal"
	"github.com/Alturino/ecommerce/internal/constants"
	inHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
)

type CartController struct {
	service *service.CartService
}

func AttachCartController(mux *mux.Router, service *service.CartService) {
	controller := CartController{service: service}

	router := mux.PathPrefix("/carts").Subrouter()
	router.Use(
		otelmux.Middleware(constants.APP_CART_SERVICE),
		middleware.Logging,
		middleware.RecoverPanic,
		middleware.Auth,
	)
	router.HandleFunc("", controller.InsertCart).Methods(http.MethodPost)
	router.HandleFunc("/{cartId}/checkout", controller.CheckoutCart).Methods(http.MethodPost)
	router.HandleFunc("/{cartId}", controller.FindCartById).Methods(http.MethodGet)
	router.HandleFunc("/{cartId}/{cartItemId}", controller.RemoveCartItem).
		Methods(http.MethodDelete)
}

func (t CartController) InsertCart(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController InsertCart")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "CartController InsertCart").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "decoding requestbody").Logger()
	logger.Trace().Msg("decoding requestbody")
	span.AddEvent("decoding requestbody")
	reqBody := request.Cart{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
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
	logger.Debug().Msg("decoded request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating requestbody").Logger()
	logger.Trace().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Debug().Msg("initialized validator")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating requestbody").Logger()
	logger.Trace().Msg("validating request body")
	span.AddEvent("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil {
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

	logger = logger.With().Str(constants.KEY_PROCESS, "getting userId from jwtToken").Logger()
	logger.Trace().Msg("getting userId from jwtToken")
	span.AddEvent("getting userId from jwtToken")
	c = logger.WithContext(c)
	userId, err := internal.UserIdFromJwtToken(c)
	if err != nil {
		err = fmt.Errorf("failed getting userId from jwtToken with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("got user id")
	logger = logger.With().Str(constants.KEY_USER_ID, userId.String()).Logger()
	logger.Debug().Msg("got user id")

	logger = logger.With().Str(constants.KEY_PROCESS, "inserting cart").Logger()
	logger.Trace().Msg("inserting cart")
	span.AddEvent("inserting cart")
	c = logger.WithContext(c)
	cart, err := t.service.InsertCart(c, reqBody, userId)
	if err != nil {
		err = fmt.Errorf("failed inserting cart with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("inserted cart")
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

func (t CartController) FindCartById(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController FindCartById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "CartController FindCartById").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "validating uuid").Logger()
	logger.Trace().Msg("validating uuid")
	span.AddEvent("validating uuid")
	pathValues := mux.Vars(r)
	cartId, err := uuid.Parse(pathValues["cartId"])
	if err != nil {
		err = fmt.Errorf("failed validating cartId=%s with error=%w", cartId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("validated uuid")
	logger = logger.With().
		Str(constants.KEY_CART_ID, cartId.String()).
		Any(constants.KEY_PATH_VALUES, pathValues).
		Logger()
	logger.Debug().Msg("validated uuid")

	logger = logger.With().Str(constants.KEY_PROCESS, "getting userId from jwtToken").Logger()
	logger.Trace().Msg("getting userId from jwtToken")
	userId, err := internal.UserIdFromJwtToken(c)
	if err != nil {
		err = fmt.Errorf("failed getting userId from jwtToken with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("got user id from jwt token")
	logger = logger.With().Str(constants.KEY_USER_ID, userId.String()).Logger()
	logger.Debug().Msg("got user id from jwt token")

	logger = logger.With().Str(constants.KEY_PROCESS, "find cart").Logger()
	logger.Trace().Msg("find cart id")
	span.AddEvent("find cart id")
	c = logger.WithContext(c)
	cart, err := t.service.FindCartById(c, request.FindCartById{ID: cartId, UserId: userId})
	if err != nil {
		err = fmt.Errorf("failed finding cartId=%s with error=%w", cartId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("found cart id")
	logger = logger.With().Any(constants.KEY_CART, cart).Logger()
	logger.Info().Msg("found cart id")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("cartId=%s found", cartId.String()),
		"data": map[string]interface{}{
			"cart": cart,
		},
	})
}

func (t CartController) RemoveCartItem(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "CartController RemoveCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "CartController RemoveCartItem").
		Str(constants.KEY_PROCESS, "validating cartId").
		Logger()

	logger.Trace().Msg("validating cartId is a valid uuid")
	cartId, err := uuid.Parse(r.PathValue("cartId"))
	if err != nil {
		err = fmt.Errorf("failed validating cartId=%s with error=%w", cartId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(constants.KEY_CART_ID, cartId.String()).Logger()
	logger.Debug().Msg("cart id is a valid uuid")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating cartItemId").Logger()
	logger.Trace().Msg("validating cartItemId is valid uuid")
	cartItemId, err := uuid.Parse(r.PathValue("cartItemId"))
	if err != nil {
		err = fmt.Errorf("failed validating cartItemId=%s with error=%w", cartId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(constants.KEY_CART_ITEM_ID, cartItemId.String()).Logger()
	logger.Debug().Msg("cart item id is a valid uuid")

	logger = logger.With().Str(constants.KEY_PROCESS, "removing cart item").Logger()
	logger.Trace().Msg("removing cart item")
	c = logger.WithContext(c)
	err = t.service.RemoveCartItem(c, request.RemoveCartItem{ID: cartItemId, CartId: cartId})
	if err != nil {
		err = fmt.Errorf("failed removing cart item with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusInternalServerError,
			"message":    err.Error(),
		})
		return
	}
}

func (t CartController) CheckoutCart(w http.ResponseWriter, r *http.Request) {
	reqId := log.RequestIDFromContext(r.Context())
	requestIdAttr := attribute.String(constants.KEY_REQUEST_ID, reqId)
	c, span := otel.Tracer.Start(
		r.Context(),
		"CartController CheckoutCart",
		trace.WithAttributes(requestIdAttr),
	)
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "CartController CheckoutCart").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "getting path values").Logger()
	logger.Trace().Msg("getting path values")
	span.AddEvent("getting path values")
	pathValues := mux.Vars(r)
	logger = logger.With().Any(constants.KEY_PATH_VALUES, pathValues).Logger()
	span.AddEvent("got path values")
	logger.Debug().Msg("got path values")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating cart id").Logger()
	logger.Trace().Msg("validating cart id")
	span.AddEvent("validating cart id")
	cartId, err := uuid.Parse(pathValues["cartId"])
	if err != nil {
		err = fmt.Errorf("failed validating cart id=%s with error=%w", cartId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("cart id is valid uuid")
	logger = logger.With().Str(constants.KEY_CART_ID, cartId.String()).Logger()
	logger.Debug().Msg("cart id is valid uuid")

	logger = logger.With().Str(constants.KEY_PROCESS, "getting userId from jwtToken").Logger()
	logger.Trace().Msg("getting userId from jwtToken")
	span.AddEvent("getting userId from jwtToken")
	userId, err := internal.UserIdFromJwtToken(c)
	if err != nil {
		err = fmt.Errorf("failed getting userId from jwtToken with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("got user id")
	logger = logger.With().Str(constants.KEY_USER_ID, userId.String()).Logger()
	logger.Debug().Msg("got user id")

	logger = logger.With().Str(constants.KEY_PROCESS, "checkout cart").Logger()
	logger.Trace().Msg("checking out cart cart")
	span.AddEvent("checking out cart cart")
	jwt := internal.JwtTokenFromContext(c)
	c = logger.WithContext(c)
	cart, err := t.service.CheckoutCart(
		c,
		jwt,
		request.CheckoutCart{UserId: userId, CartId: cartId},
	)
	if err != nil {
		err = fmt.Errorf("failed checkout cart id=%s with error=%w", cartId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("checkout cart")
	logger.Info().Msg("checkout cart")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("checkout cart id=%s", cartId.String()),
		"data": map[string]interface{}{
			"cart": cart,
		},
	})
}
