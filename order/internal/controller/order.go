package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common/errors"
	inHttp "github.com/Alturino/ecommerce/internal/common/http"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/order/internal/common/otel"
	"github.com/Alturino/ecommerce/order/internal/request"
	"github.com/Alturino/ecommerce/order/internal/service"
)

type OrderController struct {
	service *service.OrderService
}

func AttachOrderController(mux *mux.Router, service *service.OrderService) {
	router := mux.PathPrefix("/orders").Subrouter()

	controller := OrderController{service: service}

	router.HandleFunc("/", controller.FindOrders).Methods("GET")
	router.HandleFunc("/", controller.InsertOrder).Methods("POST")
	router.HandleFunc("/{orderId}", controller.FindOrderById).Methods("GET")
}

func (s *OrderController) InsertOrder(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController InsertOrder")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderController InsertOrder").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "decoding requestbody").Logger()
	logger.Info().Msg("decoding requestbody")
	reqBody := request.InsertOrder{}
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
		logger.Error().Err(err).Msg(err.Error())
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
	cart, err := s.service.InsertOrder(c, reqBody)
	if err != nil {
		err = fmt.Errorf("failed inserting order with error=%w", err)
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

func (s *OrderController) FindOrderById(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController FindOrders")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderController FindOrderById").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "validating orderId").Logger()
	logger.Info().Msg("validating orderId")
	orderId, err := uuid.Parse(r.PathValue("orderId"))
	if err != nil {
		err = fmt.Errorf("failed validating orderId=%s with error=%w", orderId.String(), err)
		errors.HandleError(err, logger, span)
		return
	}
	logger.Info().Msg("validated orderId")

	logger = logger.With().Str(log.KeyProcess, "finding orders").Logger()
	logger.Info().Msg("finding orders")
	c = logger.WithContext(c)
	orders, err := s.service.FindOrderById(c, request.FindOrderById{OrderId: orderId})
	if err != nil {
		errors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    fmt.Sprintf("order with id=%s and not found", orderId.String()),
		})
		return
	}
	logger = logger.With().Any(log.KeyOrders, orders).Logger()
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
		errors.HandleError(err, logger, span)
		return
	}
	logger.Info().Msg("validated userId")

	logger.Info().Msg("validating orderId")
	orderId, err := uuid.Parse(r.URL.Query().Get("orderId"))
	if err != nil {
		err = fmt.Errorf("failed validating orderId=%s with error=%w", orderId.String(), err)
		errors.HandleError(err, logger, span)
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
		errors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    fmt.Sprintf("order with id=%s and userId=%s not found", orderId, userId),
		})
		return
	}
	logger.Info().Any(log.KeyOrders, orders).Msg("found orders")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "found orders",
		"data": map[string]interface{}{
			"orders": orders,
		},
	})
}
