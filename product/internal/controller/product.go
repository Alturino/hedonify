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
	"github.com/Alturino/ecommerce/internal/common/response"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/product/internal/common/otel"
	"github.com/Alturino/ecommerce/product/internal/service"
	"github.com/Alturino/ecommerce/product/request"
)

type ProductController struct {
	service *service.ProductService
}

func AttachProductController(mux *mux.Router, service *service.ProductService) {
	controller := ProductController{service}

	router := mux.NewRoute().Subrouter()
	router.HandleFunc("/products", controller.InsertProduct).Methods("POST")
	router.HandleFunc("/products", controller.FindProducts).Methods("GET")
}

func (p *ProductController) InsertProduct(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController InsertProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductController InsertProduct").
		Logger()
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "decoding requestbody").
		Msg("decoding requestbody")
	reqBody := request.InsertProductRequest{}
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
		Str(log.KeyProcess, "inserting product").
		Msg("inserting product")
	product, err := p.service.InsertProduct(c, reqBody)
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
		Str(log.KeyProcess, "inserting product").
		Msg("inserted product")
	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully inserted product",
		"data": map[string]interface{}{
			"product": product,
		},
	})
}

func (p *ProductController) FindProducts(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductController FindProducts").
		Logger()
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "get query params").
		Msg("checking query params")
	id := r.URL.Query().Get("id")
	name := r.URL.Query().Get("name")
	logger = logger.With().
		Str(common.QueryParamID, id).
		Str(common.QueryParamName, name).
		Logger()
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "validating id").
		Msgf("validating id=%s", id)
	uuid, err := uuid.Parse(id)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "validating id").
			Msgf("id=%s is not uuid", id)
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    fmt.Sprintf("id=%s is not a valid uuid", id),
		})
		return
	}
	logger.Info().
		Str(log.KeyProcess, "validating id").
		Msgf("id=%s is valid uuid", id)

	logger.Info().
		Str(log.KeyProcess, "finding products by id or name").
		Msgf("finding products by id=%s or name=%s", id, name)
	products, err := p.service.FindProducts(c, request.FindProductRequest{ID: uuid, Name: name})
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "finding products by id or name").
			Msgf("id=%s is not uuid", id)
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    fmt.Sprintf("products by id=%s or name=%s not found", id, name),
		})
		return
	}
	logger.Info().
		Str(log.KeyProcess, "finding products by id or name").
		Any("products", products).
		Msgf("found products by id=%s or name=%s", id, name)
	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("products by id=%s or name=%s not found", id, name),
		"data": map[string]interface{}{
			"products": products,
		},
	})
}
