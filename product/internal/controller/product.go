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

	inErrors "github.com/Alturino/ecommerce/internal/common/errors"
	inHttp "github.com/Alturino/ecommerce/internal/common/http"
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
	router.HandleFunc("/products", controller.FindProducts).Methods(http.MethodGet)
	router.HandleFunc("/products", controller.InsertProduct).Methods(http.MethodPost)
	router.HandleFunc("/products/{productId}", controller.FindProductById).Methods(http.MethodGet)
	router.HandleFunc("/products/{productId}", controller.RemoveProduct).Methods(http.MethodDelete)
	router.HandleFunc("/products/{productId}", controller.UpdateProduct).Methods(http.MethodPut)
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
		inErrors.HandleError(err, logger, span)
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("inserted product")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
	minPrice := r.URL.Query().Get("minPrice")
	maxPrice := r.URL.Query().Get("maxPrice")
	logger = logger.With().
		Str(log.KeyProductName, name).
		Str(log.KeyMinPrice, minPrice).
		Str(log.KeyMaxPrice, maxPrice).
		Logger()
	logger.Info().Msg("get query params")

	logger = logger.With().Str(log.KeyProcess, "validating query params").Logger()
	logger.Info().Msg("validating query params")
	req := request.FindProduct{Name: name, MinPrice: minPrice, MaxPrice: maxPrice}
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.StructCtx(c, req)
	if err != nil {
		err = fmt.Errorf("failed validating query params with error=%w", err)
		inErrors.HandleError(err, logger, span)
		return
	}
	logger.Info().Msg("validated query params")

	logger = logger.With().Str(log.KeyProcess, "finding products").Logger()
	logger.Info().Msg("finding products")
	c = logger.WithContext(c)
	products, err := p.service.FindProducts(c, req)
	if err != nil {
		err = fmt.Errorf("failed finding products with name=%s with error=%w", name, err)
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyProducts, products).Logger()
	logger.Info().Msgf("found products name=%s", name)

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
	id, err := uuid.Parse(r.URL.Query().Get("productId"))
	if err != nil {
		err = fmt.Errorf("failed get product id with error=%w", err)
		inErrors.HandleError(err, logger, span)
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyProduct, product).Logger()
	logger.Info().Msgf("found product id=%s", id.String())

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(log.KeyRequestBody, reqBody).Logger()
	span.SetAttributes(
		attribute.String(log.KeyProductName, reqBody.Name),
		attribute.String(log.KeyProductPrice, reqBody.Price),
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("removed product")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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

	logger = logger.With().Str(log.KeyProcess, "getting pathvalue productId").Logger()
	logger.Info().Msg("getting pathvalue productId")
	id, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		err = fmt.Errorf("failed getting pathvalue productId with error=%w", err)
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(log.KeyProductID, id.String()).Logger()
	logger.Info().Msgf("got pathvalue productId=%s", id.String())

	logger = logger.With().Str(log.KeyProcess, "decoding request body").Logger()
	logger.Info().Msg("decoding request body")
	reqBody := request.Product{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		err = fmt.Errorf("failed decoding request body with error=%w", err)
		inErrors.HandleError(err, logger, span)
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		inErrors.HandleError(err, logger, span)
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger.Info().Msg("updated product")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully inserted product",
		"data": map[string]interface{}{
			"product": product,
		},
	})
}
