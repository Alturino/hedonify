package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	commonHttp "github.com/Alturino/ecommerce/internal/common/http"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/order/internal/common/otel"
	"github.com/Alturino/ecommerce/order/internal/request"
	"github.com/Alturino/ecommerce/order/internal/service"
	commonRequest "github.com/Alturino/ecommerce/order/pkg/request"
)

type OrderController struct {
	service *service.OrderService
}

func AttachOrderController(mux *mux.Router, service *service.OrderService) {
	controller := OrderController{service: service}

	router := mux.PathPrefix("/orders").Subrouter()
	router.HandleFunc("", controller.FindOrders).Methods(http.MethodGet)
	router.HandleFunc("/{orderId}", controller.FindOrderById).Methods(http.MethodGet)
	router.HandleFunc("/checkout", controller.CreateOrder).Methods(http.MethodPost)
}

func (s *OrderController) FindOrderById(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController FindOrders")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderController FindOrderById").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "validating orderId").Logger()
	logger.Info().Msg("validating orderId")
	pathValues := mux.Vars(r)
	orderId, err := uuid.Parse(pathValues["orderId"])
	if err != nil {
		err = fmt.Errorf("failed validating orderId=%s with error=%w", orderId.String(), err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("validated orderId")

	logger = logger.With().Str(log.KeyProcess, "finding orders").Logger()
	logger.Info().Msg("finding orders")
	c = logger.WithContext(c)
	orders, err := s.service.FindOrderById(c, request.FindOrderById{OrderId: orderId})
	if err != nil {

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    fmt.Sprintf("order with id=%s and not found", orderId.String()),
		})
		return
	}
	logger = logger.With().Any(log.KeyOrders, orders).Logger()
	logger.Info().Msg("found orders")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "found orders",
		"data": map[string]interface{}{
			"orders": orders,
		},
	})
}

func (s *OrderController) FindOrders(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController FindOrders")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderController FindOrders").
		Str(log.KeyProcess, "finding orders").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "validating userId and orderId").Logger()
	logger.Info().Msg("validating userId")
	userId, err := uuid.Parse(r.URL.Query().Get("userId"))
	if err != nil {
		err = fmt.Errorf("failed validating userId=%s with error=%w", userId.String(), err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().
		Str(log.KeyOrderID, orderId.String()).
		Str(log.KeyUserID, userId.String()).
		Logger()
	logger.Info().Msg("validated orderId")

	logger = logger.With().Str(log.KeyProcess, "finding orders").Logger()
	logger.Info().Msg("finding orders")
	c = logger.WithContext(c)
	orders, err := s.service.FindOrders(c, request.FindOrders{OrderId: orderId, UserId: userId})
	if err != nil {
		err = fmt.Errorf("failed finding orders with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Any(log.KeyOrders, orders).Msg("found orders")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "found orders",
		"data": map[string]interface{}{
			"orders": orders,
		},
	})
}

func (s *OrderController) CreateOrder(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController CreateOrder")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderController CreateOrder").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "getting userId from jwtToken").Logger()
	logger.Info().Msg("getting userId from jwtToken")
	userId, err := common.UserIdFromJwtToken(c)
	if err != nil {
		err = fmt.Errorf("failed getting userId from jwtToken with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(log.KeyUserID, userId.String()).Logger()
	logger.Info().Msgf("got userId=%s", userId.String())

	logger = logger.With().Str(log.KeyProcess, "decoding request body").Logger()
	logger.Info().Msg("decoding request body")
	param := commonRequest.CreateOrder{}
	err = json.NewDecoder(r.Body).Decode(&param)
	if err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    "request body is invalid",
		})
		return
	}
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(log.KeyProcess, "validating request body").Logger()
	logger.Info().Msg("validating request body")
	validate := validator.New(validator.WithRequiredStructEnabled())
	err = validate.StructCtx(c, param)
	if err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    "request body is invalid",
		})
		return
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(log.KeyProcess, "creating order").Logger()
	logger.Info().Msg("creating order")
	c = logger.WithContext(c)
	order, err := s.service.CreateOrder(c, param)
	if err != nil {
		err = fmt.Errorf("failed creating order with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusInternalServerError,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("created order")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "order created",
		"data": map[string]interface{}{
			"order": order,
		},
	})
}
