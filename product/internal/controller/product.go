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

	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	commonHttp "github.com/Alturino/ecommerce/internal/common/http"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/product/internal/common/otel"
	"github.com/Alturino/ecommerce/product/internal/service"
	"github.com/Alturino/ecommerce/product/pkg/request"
)

type ProductController struct {
	service *service.ProductService
}

func AttachProductController(mux *mux.Router, service *service.ProductService) {
	controller := ProductController{service}

	router := mux.PathPrefix("/products").Subrouter()
	router.HandleFunc("", controller.GetProducts).Methods(http.MethodGet)

	postRouter := mux.PathPrefix("/products").Methods(http.MethodPost).Subrouter()
	postRouter.HandleFunc("", controller.InsertProduct).Methods(http.MethodPost)
	postRouter.Use(middleware.Auth)

	router.HandleFunc("/{productId}", controller.FindProductById).Methods(http.MethodGet)
	router.HandleFunc("/{productId}", controller.RemoveProduct).Methods(http.MethodDelete)
	router.HandleFunc("/{productId}", controller.UpdateProduct).Methods(http.MethodPut)
}

func (p ProductController) InsertProduct(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController InsertProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KEY_TAG, "ProductController InsertProduct").
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "decoding request body").Logger()
	logger.Info().Msg("decoding request body")
	reqBody := request.Product{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(log.KEY_PROCESS, "validating.request_body").Logger()

	logger.Info().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().Msg("initialized validator")

	logger.Info().Msg("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil {
		err = fmt.Errorf("failed validating request body with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(log.KEY_PROCESS, "inserting product").Logger()
	logger.Info().Msg("inserting product")
	c = logger.WithContext(c)
	product, err := p.service.InsertProduct(c, reqBody)
	if err != nil {
		err = fmt.Errorf("failed inserting product with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("inserted product")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully inserted product",
		"data": map[string]interface{}{
			"product": product,
		},
	})
}

func (ctrl ProductController) GetProducts(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KEY_TAG, "ProductController FindProducts").
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "get products").Logger()
	logger.Info().Msg("get products")
	span.AddEvent("get products")
	c = logger.WithContext(c)
	products, err := ctrl.service.GetProducts(c)
	if err != nil {
		err = fmt.Errorf("failed get products with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KEY_PRODUCTS, products).Logger()
	span.AddEvent("got products")
	logger.Info().Msg("got products")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "products found",
		"data": map[string]interface{}{
			"products": products,
		},
	})
}

func (p ProductController) FindProductById(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController FindProductById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KEY_TAG, "ProductController FindProductById").
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "get product id").Logger()
	logger.Info().Msg("get product id")
	id, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		err = fmt.Errorf("failed get product id with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger = logger.With().Str(log.KEY_PRODUCT_ID, id.String()).Logger()
	span.SetAttributes(attribute.String(log.KEY_PRODUCT_ID, id.String()))
	logger.Info().Msgf("got product id=%s", id.String())

	logger = logger.With().Str(log.KEY_PROCESS, "finding product").Logger()
	logger.Info().Msg("finding product")
	c = logger.WithContext(c)
	product, err := p.service.FindProductById(c, id)
	if err != nil {
		err = fmt.Errorf("failed finding product with id=%s with error=%w", id.String(), err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KEY_PRODUCT, product).Logger()
	logger.Info().Msgf("found product id=%s", id.String())

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("product id=%s found", id.String()),
		"data": map[string]interface{}{
			"product": product,
		},
	})
}

func (p ProductController) RemoveProduct(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController RemoveProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KEY_TAG, "ProductController RemoveProduct").
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "getting pathvalue productId").Logger()
	logger.Info().Msg("getting pathvalue productId")
	id, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		err = fmt.Errorf("failed getting pathvalue productId with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(log.KEY_PRODUCT_ID, id.String()).Logger()
	span.SetAttributes(attribute.String(log.KEY_PRODUCT_ID, id.String()))
	logger.Info().Msgf("got pathvalue productId=%s", id.String())

	logger.Info().Msg("decoding request body")
	reqBody := request.Product{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}

	span.SetAttributes(
		attribute.String(log.KEY_PRODUCT_NAME, reqBody.Name),
		attribute.String(log.KEY_PRODUCT_PRICE, reqBody.Price.String()),
		attribute.Int(log.KEY_PRODUCT_QUANTITY, reqBody.Quantity),
	)
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(log.KEY_PROCESS, "validating.request_body").Logger()

	logger.Info().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().Msg("initialized validator")

	logger.Info().Msg("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil {
		err = fmt.Errorf("failed validating request body with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(log.KEY_PROCESS, "remove product").Logger()
	logger.Info().Msg("remove product")
	product, err := p.service.RemoveProduct(c, id)
	if err != nil {
		err = fmt.Errorf("failed remove product with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("removed product")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully inserted product",
		"data": map[string]interface{}{
			"product": product,
		},
	})
}

func (p ProductController) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController UpdateProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KEY_TAG, "ProductController UpdateProduct").
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "getting pathValue productId").Logger()
	logger.Info().Msg("getting pathValue productId")
	id, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		err = fmt.Errorf("failed getting pathValue productId with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(log.KEY_PRODUCT_ID, id.String()).Logger()
	logger.Info().Msgf("got pathValue productId=%s", id.String())

	logger = logger.With().Str(log.KEY_PROCESS, "decoding request body").Logger()
	logger.Info().Msg("decoding request body")
	reqBody := request.Product{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}

	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(log.KEY_PROCESS, "validating.request_body").Logger()

	logger.Info().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().Msg("initialized validator")

	logger.Info().Msg("validating request body")
	if err := validate.StructCtx(c, reqBody); err != nil {
		err = fmt.Errorf("failed validating request body with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(log.KEY_PROCESS, "updating product").Logger()
	logger.Info().Msg("updating product")
	product, err := p.service.UpdateProduct(c, id, reqBody)
	if err != nil {
		err = fmt.Errorf("failed updating product with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("updated product")

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully updated product",
		"data": map[string]interface{}{
			"product": product,
		},
	})
}
