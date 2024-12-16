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

func (p ProductController) InsertProduct(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController InsertProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductController InsertProduct").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "decoding request body").Logger()
	logger.Info().Msg("decoding request body")
	reqBody := request.InsertProductRequest{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
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
		logger.Error().Err(err).Msg(err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(log.KeyProcess, "inserting product").Logger()
	logger.Info().Msg("inserting product")
	c = logger.WithContext(c)
	product, err := p.service.InsertProduct(c, reqBody)
	if err != nil {
		err = fmt.Errorf("failed inserting product with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("inserted product")

	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully inserted product",
		"data": map[string]interface{}{
			"product": product,
		},
	})
}

func (p ProductController) FindProducts(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductController FindProducts").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "validating query params").Logger()
	logger.Info().Msg("validating query params")
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		err = fmt.Errorf("failed parsing uuid with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    "id is not valid uuid",
		})
		return
	}
	name := r.URL.Query().Get("name")
	logger = logger.With().
		Str(common.QueryParamID, id.String()).
		Str(common.QueryParamName, name).
		Logger()
	logger.Info().Msg("validated query params")

	logger = logger.With().Str(log.KeyProcess, "finding products").Logger()
	logger.Info().Msg("finding products")
	products, err := p.service.FindProducts(c, request.FindProductRequest{ID: id, Name: name})
	if err != nil {
		err = fmt.Errorf(
			"failed finding products with id=%s and name=%s with error=%w",
			id.String(),
			name,
			err,
		)
		logger.Error().Err(err).Msg(err.Error())
		response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyProducts, products).Logger()
	logger.Info().Msgf("found products by id=%s or name=%s", id, name)
	response.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("products by id=%s or name=%s found", id, name),
		"data": map[string]interface{}{
			"products": products,
		},
	})
}
