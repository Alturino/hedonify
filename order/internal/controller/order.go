package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common/response"
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

	logger.Info().
		Str(log.KeyProcess, "decoding requestBody").
		Msg("decoding requestBody")
	reqBody := request.InsertOrderRequest{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		logger.Error().Err(err).
			Str(log.KeyProcess, "decoding requestBody").
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
	logger.Info().
		Str(log.KeyProcess, "decoding requestBody").
		Msg("decoded request body")

	logger.Info().
		Str(log.KeyProcess, "validating requestBody").
		Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().
		Str(log.KeyProcess, "validating requestBody").
		Msg("initialized validator")
	logger.Info().
		Str(log.KeyProcess, "validating requestBody").
		Msg("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil {
		logger.Error().Err(err).
			Str(log.KeyProcess, "validating requestBody").
			Msgf("failed validating request body with error=%s", err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().
		Str(log.KeyProcess, "validating requestBody").
		Msg("validated request body")

	logger = logger.With().
		Str(log.KeyProcess, "inserting cart").
		Logger()
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "inserting cart").
		Msg("inserting cart")
	cart, err := s.service.InsertOrder(c, reqBody)
	if err != nil {
		err = fmt.Errorf("failed inserting order with error=%w", err)
		logger.Error().Err(err).
			Str(log.KeyProcess, "inserting cart").
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

func (s *OrderController) FindOrderById(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "OrderController FindOrders")
	defer span.End()

	orderId, err := uuid.Parse(r.PathValue("orderId"))
	if err != nil {
	}

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderController FindOrderById").
		Str(log.KeyProcess, "finding orders").
		Str(log.KeyOrderID, orderId.String()).
		Logger()
	c = logger.WithContext(c)

	logger.Info().Msg("finding orders")
	orders, err := s.service.FindOrderById(c, request.FindOrderById{OrderId: orderId})
	if err != nil {
		logger.Error().Err(err).Msg(err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    fmt.Sprintf("order with id=%s and not found", orderId.String()),
		})
		return
	}
	logger.Info().Any(log.KeyOrders, orders).Msg("found orders")

	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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

	userId := r.URL.Query().Get("userId")
	orderId := r.URL.Query().Get("orderId")

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderController FindOrders").
		Str(log.KeyProcess, "finding orders").
		Str(log.KeyUserID, userId).
		Str(log.KeyOrderID, orderId).
		Logger()
	c = logger.WithContext(c)

	logger.Info().Msg("finding orders")
	orders, err := s.service.FindOrders(c, request.FindOrders{OrderId: orderId, UserId: userId})
	if err != nil {
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    fmt.Sprintf("order with id=%s and userId=%s not found", orderId, userId),
		})
		return
	}
	logger.Info().Any(log.KeyOrders, orders).Msg("found orders")

	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "found orders",
		"data": map[string]interface{}{
			"orders": orders,
		},
	})
}
