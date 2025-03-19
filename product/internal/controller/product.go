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

	"github.com/Alturino/ecommerce/internal/constants"
	inHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/middleware"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/product/internal/otel"
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
	router.Use(
		otelmux.Middleware(constants.APP_PRODUCT_SERVICE),
		middleware.Logging,
		middleware.RecoverPanic,
		middleware.Auth,
	)
	router.HandleFunc("/{productId}", controller.FindProductById).Methods(http.MethodGet)
	router.HandleFunc("/{productId}", controller.RemoveProduct).Methods(http.MethodDelete)
	router.HandleFunc("/{productId}", controller.UpdateProduct).Methods(http.MethodPut)
}

func (p ProductController) InsertProduct(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController InsertProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "ProductController InsertProduct").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "decoding request body").Logger()
	logger.Trace().Msg("decoding request body")
	span.AddEvent("decoding request body")
	reqBody := request.Product{}
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
	logger.Trace().Msg("decoded request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating.request_body").Logger()
	logger.Trace().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Trace().Msg("initialized validator")

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
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "inserting product").Logger()
	logger.Info().Msg("inserting product")
	c = logger.WithContext(c)
	product, err := p.service.InsertProduct(c, reqBody)
	if err != nil {
		err = fmt.Errorf("failed inserting product with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
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

func (ctrl ProductController) GetProducts(w http.ResponseWriter, r *http.Request) {
	c, span := otel.Tracer.Start(r.Context(), "ProductController FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "ProductController FindProducts").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "get products").Logger()
	logger.Trace().Msg("get products")
	span.AddEvent("get products")
	c = logger.WithContext(c)
	products, err := ctrl.service.GetProducts(c)
	if err != nil {
		err = fmt.Errorf("failed get products with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Any(constants.KEY_PRODUCTS, products).Logger()
	span.AddEvent("got products")
	logger.Info().Msg("got products")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
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
		Ctx(c).
		Str(constants.KEY_TAG, "ProductController FindProductById").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "get product id").Logger()
	logger.Trace().Msg("get product id from pathValues")
	span.AddEvent("get product id from pathValues")
	pathValues := mux.Vars(r)
	id, err := uuid.Parse(pathValues["productId"])
	if err != nil {
		err = fmt.Errorf("failed get product id from pathValues with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(constants.KEY_PRODUCT_ID, id.String()).Logger()
	logger.Trace().Msg("got product id")

	logger = logger.With().Str(constants.KEY_PROCESS, "finding product").Logger()
	logger.Trace().Msg("finding product")
	span.AddEvent("finding product")
	c = logger.WithContext(c)
	product, err := p.service.FindProductById(c, id)
	if err != nil {
		err = fmt.Errorf("failed finding product with id=%s with error=%w", id.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("found product id")
	logger = logger.With().Any(constants.KEY_PRODUCT, product).Logger()
	logger.Info().Msg("found product id")

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
		Ctx(c).
		Str(constants.KEY_TAG, "ProductController RemoveProduct").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "getting pathvalue productId").Logger()
	logger.Info().Msg("getting pathvalue productId")
	id, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		err = fmt.Errorf("failed getting pathvalue productId with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	logger = logger.With().Str(constants.KEY_PRODUCT_ID, id.String()).Logger()
	span.SetAttributes(attribute.String(constants.KEY_PRODUCT_ID, id.String()))
	logger.Info().Msgf("got pathvalue productId=%s", id.String())

	logger.Info().Msg("decoding request body")
	reqBody := request.Product{}
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

	span.SetAttributes(
		attribute.String(constants.KEY_PRODUCT_NAME, reqBody.Name),
		attribute.String(constants.KEY_PRODUCT_PRICE, reqBody.Price.String()),
		attribute.Int(constants.KEY_PRODUCT_QUANTITY, reqBody.Quantity),
	)
	logger.Info().Msg("decoded request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "validating.request_body").Logger()

	logger.Info().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().Msg("initialized validator")

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
	logger.Info().Msg("validated request body")

	logger = logger.With().Str(constants.KEY_PROCESS, "remove product").Logger()
	logger.Trace().Msg("remove product")
	span.AddEvent("remove product")
	product, err := p.service.RemoveProduct(c, id)
	if err != nil {
		err = fmt.Errorf("failed remove product with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("removed product")
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
		Ctx(c).
		Str(constants.KEY_TAG, "ProductController UpdateProduct").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "getting pathValue productId").Logger()
	logger.Trace().Msg("getting pathValue productId")
	span.AddEvent("getting pathValue productId")
	id, err := uuid.Parse(r.PathValue("productId"))
	if err != nil {
		err = fmt.Errorf("failed getting pathValue productId with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("got pathValue")
	logger = logger.With().Str(constants.KEY_PRODUCT_ID, id.String()).Logger()
	logger.Debug().Msg("got pathValue")

	logger = logger.With().Str(constants.KEY_PROCESS, "decoding request body").Logger()
	logger.Trace().Msg("decoding request body")
	span.AddEvent("decoding request body")
	reqBody := request.Product{}
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

	logger = logger.With().Str(constants.KEY_PROCESS, "validating.request_body").Logger()
	logger.Trace().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Trace().Msg("initialized validator")
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

	logger = logger.With().Str(constants.KEY_PROCESS, "updating product").Logger()
	logger.Trace().Msg("updating product")
	span.AddEvent("updating product")
	c = logger.WithContext(c)
	product, err := p.service.UpdateProduct(c, id, reqBody)
	if err != nil {
		err = fmt.Errorf("failed updating product with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
			"status":     "failed",
			"statusCode": http.StatusBadRequest,
			"message":    err.Error(),
		})
		return
	}
	span.AddEvent("updated product")
	logger.Debug().Msg("updated product")

	inHttp.WriteJsonResponse(c, w, map[string]string{}, map[string]interface{}{
		"status":     "success",
		"statusCode": http.StatusOK,
		"message":    "successfully updated product",
		"data": map[string]interface{}{
			"product": product,
		},
	})
}
