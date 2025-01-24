package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"

	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	commonHttp "github.com/Alturino/ecommerce/internal/common/http"
	commonValidate "github.com/Alturino/ecommerce/internal/common/validate"
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

	router := mux.PathPrefix("/products").Subrouter()
	router.HandleFunc("", controller.FindProducts).Methods(http.MethodGet)
	router.HandleFunc("", controller.InsertProduct).Methods(http.MethodPost)
	router.HandleFunc("/{productId}", controller.FindProductById).Methods(http.MethodGet)
	router.HandleFunc("/{productId}", controller.RemoveProduct).Methods(http.MethodDelete)
	router.HandleFunc("/{productId}", controller.UpdateProduct).Methods(http.MethodPut)
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

	logger = logger.With().Str(log.KeyProcess, "validating requestbody").Logger()
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

	logger = logger.With().Str(log.KeyProcess, "inserting product").Logger()
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

func (p ProductController) FindProducts(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductController FindProducts").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "get query params").Logger()
	logger.Info().Msg("get query params")
	name := r.URL.Query().Get("name")
	minPrice, err := decimal.NewFromString(r.URL.Query().Get("minPrice"))
	if err != nil {
		err = fmt.Errorf("failed to validate minPrice with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		commonErrors.HandleError(err, span)
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	maxPrice, err := decimal.NewFromString(r.URL.Query().Get("maxPrice"))
	if err != nil {
		err = fmt.Errorf("failed validating maxPrice with error=%w", err)
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
		Str(log.KeyProductName, name).
		Str(log.KeyMinPrice, minPrice.String()).
		Str(log.KeyMaxPrice, maxPrice.String()).
		Logger()
	logger.Info().Msg("get query params")

	logger = logger.With().Str(log.KeyProcess, "validating query params").Logger()
	logger.Info().Msg("validating query params")
	req := request.FindProduct{Name: name, MinPrice: minPrice, MaxPrice: maxPrice}
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterCustomTypeFunc(commonValidate.PriceValue, decimal.Decimal{})
	err = validate.StructCtx(c, req)
	if err != nil {
		err = fmt.Errorf("failed validating query params with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("validated query params")

	logger = logger.With().Str(log.KeyProcess, "finding products").Logger()
	logger.Info().Msg("finding products")
	c = logger.WithContext(c)
	products, err := p.service.FindProducts(c, req)
	if err != nil {
		err = fmt.Errorf("failed finding products with name=%s with error=%w", name, err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyProducts, products).Logger()
	logger.Info().Msgf("found products name=%s", name)

	commonHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    fmt.Sprintf("products or name=%s found", name),
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
		Str(log.KeyTag, "ProductController FindProductById").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "get product id").Logger()
	logger.Info().Msg("get product id")
	id, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		err = fmt.Errorf("failed get product id with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger = logger.With().Str(log.KeyProductID, id.String()).Logger()
	span.SetAttributes(attribute.String(log.KeyProductID, id.String()))
	logger.Info().Msgf("got product id=%s", id.String())

	logger = logger.With().Str(log.KeyProcess, "finding product").Logger()
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
	logger = logger.With().Any(log.KeyProduct, product).Logger()
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
		Str(log.KeyTag, "ProductController RemoveProduct").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "getting pathvalue productId").Logger()
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
	logger = logger.With().Str(log.KeyProductID, id.String()).Logger()
	span.SetAttributes(attribute.String(log.KeyProductID, id.String()))
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
		attribute.String(log.KeyProductName, reqBody.Name),
		attribute.String(log.KeyProductPrice, reqBody.Price.String()),
		attribute.Int(log.KeyProductQuantity, reqBody.Quantity),
	)
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(log.KeyProcess, "validating requestbody").Logger()
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

	logger = logger.With().Str(log.KeyProcess, "remove product").Logger()
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
		Str(log.KeyTag, "ProductController UpdateProduct").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "getting pathValue productId").Logger()
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
	logger = logger.With().Str(log.KeyProductID, id.String()).Logger()
	logger.Info().Msgf("got pathValue productId=%s", id.String())

	logger = logger.With().Str(log.KeyProcess, "decoding request body").Logger()
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

	logger = logger.With().Str(log.KeyProcess, "validating requestbody").Logger()
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

	logger = logger.With().Str(log.KeyProcess, "updating product").Logger()
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
