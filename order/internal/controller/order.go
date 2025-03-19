package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/internal"
	"github.com/Alturino/ecommerce/internal/constants"
	inHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/middleware"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/order/internal/otel"
	"github.com/Alturino/ecommerce/order/internal/response"
	"github.com/Alturino/ecommerce/order/internal/service"
	"github.com/Alturino/ecommerce/order/pkg/request"
)

type OrderController struct {
	service *service.OrderService
	queue   chan<- request.CreateOrder
}

func AttachOrderController(
	mux *mux.Router,
	service *service.OrderService,
	queue chan<- request.CreateOrder,
) {
	controller := OrderController{service: service, queue: queue}

	router := mux.PathPrefix("/orders").Subrouter()
	router.Use(
		otelmux.Middleware(constants.APP_ORDER_SERVICE),
		middleware.Logging,
		middleware.RecoverPanic,
		middleware.Auth,
	)
	router.HandleFunc("", controller.FindOrders).Methods(http.MethodGet)
	router.HandleFunc("/{orderId}", controller.FindOrderById).Methods(http.MethodGet)
	router.HandleFunc("/checkout", controller.Checkout).Methods(http.MethodPost)
	// router.HandleFunc("/checkout", controller.CreateOrderOptimisticLock).Methods(http.MethodPost)
}

func (ctrl OrderController) FindOrderById(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController FindOrders")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "OrderController FindOrderById").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "validating orderId").Logger()
	logger.Info().Msg("validating orderId")
	pathValues := mux.Vars(r)
	orderId, err := uuid.Parse(pathValues["orderId"])
	if err != nil {
		err = fmt.Errorf("failed validating orderId=%s with error=%w", orderId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("validated orderId")

	logger = logger.With().Str(constants.KEY_PROCESS, "finding orders").Logger()
	logger.Info().Msg("finding orders")
	c = logger.WithContext(c)
	orders, err := ctrl.service.FindOrderById(c, request.FindOrderById{OrderId: orderId})
	if err != nil {
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    fmt.Sprintf("order with id=%s and not found", orderId.String()),
		})
		return
	}
	logger = logger.With().Any(constants.KEY_ORDERS, orders).Logger()
	logger.Info().Msg("found orders")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "found orders",
		"data": map[string]interface{}{
			"orders": orders,
		},
	})
}

func (ctrl OrderController) FindOrders(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController FindOrders")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "OrderController FindOrders").
		Str(constants.KEY_PROCESS, "finding orders").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "validating userId and orderId").Logger()
	logger.Info().Msg("validating userId")
	userId, err := uuid.Parse(r.URL.Query().Get("userId"))
	if err != nil {
		err = fmt.Errorf("failed validating userId=%s with error=%w", userId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})

		return
	}
	logger.Info().Msg("validated userId")

	logger.Info().Msg("validating orderId")
	orderId, err := uuid.Parse(r.URL.Query().Get("orderId"))
	if err != nil {
		err = fmt.Errorf("failed validating orderId=%s with error=%w", orderId.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})

		return
	}
	logger = logger.With().
		Str(constants.KEY_ORDER_ID, orderId.String()).
		Str(constants.KEY_USER_ID, userId.String()).
		Logger()
	logger.Info().Msg("validated orderId")

	logger = logger.With().Str(constants.KEY_PROCESS, "finding orders").Logger()
	logger.Info().Msg("finding orders")
	c = logger.WithContext(c)
	orders, err := ctrl.service.FindOrders(
		c,
		request.FindOrders{OrderId: orderId, UserId: userId},
	)
	if err != nil {
		err = fmt.Errorf("failed finding orders with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})

		return
	}
	logger.Info().Any(constants.KEY_ORDERS, orders).Msg("found orders")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "found orders",
		"data": map[string]interface{}{
			"orders": orders,
		},
	})
}

func (ctrl OrderController) Checkout(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController BatchCreateOrder")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "OrderController BatchCreateOrder").
		Logger()

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
	span.AddEvent("got userId from jwtToken")
	logger = logger.With().Str(constants.KEY_USER_ID, userId.String()).Logger()
	logger.Info().Msg("got userId from jwtToken")

	logger = logger.With().Str(constants.KEY_PROCESS, "decoding request body").Logger()
	logger.Trace().Msg("decoding request body")
	param := request.CreateOrder{}
	span.AddEvent("decoding request body")
	err = json.NewDecoder(r.Body).Decode(&param)
	if err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    "request body is invalid",
		})
		return
	}
	span.AddEvent("decoded request body")
	logger = logger.With().
		Str(constants.KEY_ORDER_ID, param.ID.String()).
		Str(constants.KEY_USER_ID, param.UserId.String()).
		Logger()
	param.TraceLink = trace.LinkFromContext(c)
	span.SetAttributes(
		attribute.String(constants.KEY_ORDER_ID, param.ID.String()),
		attribute.String(constants.KEY_USER_ID, param.UserId.String()),
	)
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating request body").Logger()
	logger.Trace().Msg("validating request body")
	validate := validator.New(validator.WithRequiredStructEnabled())
	span.AddEvent("validating request body")
	err = validate.StructCtx(c, param)
	if err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    "request body is invalid",
		})
		return
	}
	span.AddEvent("validated request body")
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "creating order").Logger()
	logger.Trace().Msg("creating order")
	param.ResultChannel = make(chan response.Result, 1)
	c = logger.WithContext(c)
	c, done := context.WithTimeoutCause(c, time.Second*3, errors.New("timeout creating order"))
	defer done()
	select {
	case <-c.Done():
		err = fmt.Errorf("failed creating order with error=%w", c.Err())
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusInternalServerError,
			"message":    err.Error(),
		})
		return
	case ctrl.queue <- param:
		logger.Info().Msg("inserted order to queue")
	}
	select {
	case <-c.Done():
		err = fmt.Errorf("failed creating order with error=%w", c.Err())
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusInternalServerError,
			"message":    err.Error(),
		})
		return
	case result := <-param.ResultChannel:
		if result.Err != nil {
			err = fmt.Errorf("failed creating order with error=%w", result.Err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
				"status":     "failed",
				"statusCode": http.StatusBadRequest,
				"message":    err.Error(),
			})
			return
		}
		logger.Info().Msg("order created")
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "success",
			"statusCode": http.StatusCreated,
			"message":    "order created",
			"data": map[string]interface{}{
				"order": result.Order,
			},
		})
		return
	}
}
