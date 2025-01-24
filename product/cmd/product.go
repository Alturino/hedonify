package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
	orderRequest "github.com/Alturino/ecommerce/order/pkg/request"
	"github.com/Alturino/ecommerce/product/internal/common/cache"
	commonOtel "github.com/Alturino/ecommerce/product/internal/common/otel"
	"github.com/Alturino/ecommerce/product/internal/controller"
	"github.com/Alturino/ecommerce/product/internal/repository"
	"github.com/Alturino/ecommerce/product/internal/service"
)

func RunProductService(c context.Context) {
	c, span := commonOtel.Tracer.Start(c, "RunProductService")
	defer span.End()

	logger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", constants.AppProductService)).
		With().
		Str(log.KeyAppName, constants.AppProductService).
		Str(log.KeyTag, "main RunProductService").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	cfg := config.InitConfig(c, constants.AppProductService)
	logger = logger.With().Any(log.KeyConfig, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	shutdownFuncs, err := otel.InitOtelSdk(c, constants.AppProductService, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		logger.Err(err).Msg(err.Error())
		commonErrors.HandleError(err, span)
		return
	}
	logger.Info().Msg("initialized otel sdk")
	defer func() {
		logger.Info().Msg("shutting down otel")
		err = otel.ShutdownOtel(c, shutdownFuncs)
		if err != nil {
			err = fmt.Errorf("failed shutting down otel with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("shutdown otel")
	}()

	logger = logger.With().Str(log.KeyProcess, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	c = logger.WithContext(c)
	db := infra.NewDatabaseClient(c, cfg.Database)
	logger.Info().Msg("initialized database")
	defer func() {
		logger := logger.With().Str(log.KeyProcess, "shutting down database connection").Logger()
		logger.Info().Msg("shutting down database connection")
		db.Close()
		logger.Info().Msg("shutdown database connection")
	}()

	logger = logger.With().Str(log.KeyProcess, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	c = logger.WithContext(c)
	cache := infra.NewCacheClient(c, cfg.Cache)
	logger.Info().Msg("initialized cache")
	defer func() {
		logger := logger.With().Str(log.KeyProcess, "shutting down cache connection").Logger()
		logger.Info().Msg("shutting down cache connection")
		span.AddEvent("shutting down cache connection")
		err := cache.Close()
		if err != nil {
			err = fmt.Errorf("failed closing cache with error=%w", err)
			logger.Error().Err(err).Msg(err.Error())
			commonErrors.HandleError(err, span)
			return
		}
		span.AddEvent("shutdown cache connection")
		logger.Info().Msg("shutdown cache connection")
	}()

	logger = logger.With().Str(log.KeyProcess, "initializing productService").Logger()
	logger.Info().Msg("initializing productService")
	queries := repository.New(db)
	productService := service.NewProductService(queries, cache)
	logger.Info().Msg("initialized productService")

	logger = logger.With().Str(log.KeyProcess, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.StrictSlash(true)
	mux.Use(otelmux.Middleware(constants.AppProductService), middleware.Logging, middleware.Auth)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(log.KeyProcess, "attach product controller").Logger()
	logger.Info().Msg("attaching product controller")
	controller.AttachProductController(mux, &productService)
	logger.Info().Msg("attached product controller")

	logger = logger.With().Str(log.KeyProcess, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	logger.Info().Msg("initialized server")
	defer func() {
		logger = logger.With().Str(log.KeyProcess, "shutting down http server").Logger()
		span.AddEvent("shutting down http server")
		logger.Info().Msg("shutting down http server")
		err = server.Shutdown(c)
		if err != nil {
			err = fmt.Errorf("failed shutting down http server with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		span.AddEvent("shutdown server")
		logger.Info().Msg("shutdown server")
	}()

	go func() {
		logger = logger.With().Str(log.KeyProcess, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
			err = fmt.Errorf("encounter error=%w while running server", err)
			logger.Error().Err(err).Msg(err.Error())
			c = logger.WithContext(c)
			if err := otel.ShutdownOtel(c, shutdownFuncs); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)
				commonErrors.HandleError(err, span)
				logger.Error().Err(err).Msg(err.Error())
			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	c = logger.WithContext(c)
	go runUpdateProductQuantityListener(c, cache, repository.New(db), db, &wg)

	<-c.Done()
	wg.Wait()
	logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")
	c = logger.WithContext(c)
	err = server.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("failed shutting down server with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("shutdown server")

	logger.Info().Msg("shutting down otel")
	c = logger.WithContext(c)
	err = otel.ShutdownOtel(c, shutdownFuncs)
	if err != nil {
		err = fmt.Errorf("failed shutting down otel with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("shutdown otel")
	logger.Info().Msg("server completely shutdown")
}

func runUpdateProductQuantityListener(
	c context.Context,
	redis *redis.Client,
	queries *repository.Queries,
	pool *pgxpool.Pool,
	wg *sync.WaitGroup,
) {
	c, span := commonOtel.Tracer.Start(c, "runUpdateProductQuantityListener")
	defer span.End()
	defer wg.Done()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "runUpdateProductQuantityListener").
		Logger()

	ticker := time.NewTicker(time.Second * 3).C

	validate := validator.New(validator.WithRequiredStructEnabled())

	channel := redis.Subscribe(c, constants.UPDATE_PRODUCT_QUANTITY).Channel()
	params := []orderRequest.OrderItem{}

	logger.Info().Msg("started update product quantity listener")

loop:
	for {
		select {
		case <-c.Done():
			break loop
		case msg := <-channel:
			logger = logger.With().Str(log.KeyProcess, "received message from publisher").Str(log.KeyMessage, msg.String()).Logger()
			logger.Info().Msg("received message from publisher")

			logger.Info().Msg("unmarshalling payload")
			param := orderRequest.CreateOrder{}
			err := json.Unmarshal([]byte(msg.Payload), &param)
			if err != nil {
				err = fmt.Errorf("failed to unmarshal payload with error=%w", err)
				commonErrors.HandleError(err, span)
				logger.Error().Err(err).Msg(err.Error())
				continue
			}
			logger.Info().Msg("unmarshalled payload")

			logger.Info().Msg("validating payload")
			err = validate.StructCtx(c, param)
			if err != nil {
				err = fmt.Errorf("failed validating payload with error=%w", err)
				commonErrors.HandleError(err, span)
				logger.Error().Err(err).Msg(err.Error())
				continue
			}
			logger.Info().Msg("validated payload")

			params = append(params, param.OrderItems...)
		case <-ticker:
			logger = logger.With().Str(log.KeyProcess, "batch update product quantity").Logger()
			logger.Info().Msg("starting batch update product quantity")
			if len(params) == 0 {
				logger.Info().Msg("params is empty, skipping update product quantity")
				continue
			}

			logger.Info().Msg("update product quantity")
			c = logger.WithContext(c)
			err := updateProductQuantity(c, redis, queries, pool, params)
			if err != nil {
				err = fmt.Errorf("failed to update product quantity with error=%w", err)
				commonErrors.HandleError(err, span)
				logger.Error().Err(err).Msg(err.Error())
				continue
			}
			logger.Info().Msg("updated product quantity")
			params = params[:0]
		}
	}
	logger.Info().Msg("stopped listener")
}

func updateProductQuantity(
	c context.Context,
	redis *redis.Client,
	queries *repository.Queries,
	pool *pgxpool.Pool,
	params []orderRequest.OrderItem,
) error {
	c, span := commonOtel.Tracer.Start(c, "updateProductQuantity")
	defer span.End()

	logger := zerolog.Ctx(c).With().Str(log.KeyTag, "updateProductQuantity").Logger()

	logger.Info().Msg("merging duplicate product")
	span.AddEvent("merging duplicate product")
	mp := map[string]orderRequest.OrderItem{}
	for _, item := range params {
		key := item.ProductID.String()
		_, ok := mp[key]
		if ok {
			continue
		}
		mp[key] = item
	}
	temp := []orderRequest.OrderItem{}
	for _, v := range mp {
		temp = append(temp, v)
	}
	params = temp
	logger.Info().Msg("merged duplicate product")
	span.AddEvent("merged duplicate product")

	logger.Info().Msg("initializing transaction")
	span.AddEvent("initializing transaction")
	tx, err := pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		err = fmt.Errorf("failed initializing transaction with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	defer func() {
		logger.Info().Msg("rolling back transaction")
		err = tx.Rollback(c)
		if err != nil {
			err = fmt.Errorf("failed rolling back transaction with error=%w", err)
			if errors.Is(err, pgx.ErrTxClosed) {
				logger.Info().Err(err).Msg(err.Error())
				return
			}
			logger.Error().Err(err).Msg(err.Error())
			commonErrors.HandleError(err, span)
			return
		}
		logger.Info().Msg("rolled back transaction")
	}()
	span.AddEvent("initialized transaction")
	logger.Info().Msg("initialized transaction")

	logger.Info().Msg("updating product quantity")
	span.AddEvent("updating product quantity")
	updatedList := make([]repository.Product, len(params))
	for i, item := range params {
		cacheKey := cache.KEY_PRODUCTS + item.ProductID.String()
		span.SetAttributes(
			attribute.String(log.KeyProductID, item.ProductID.String()),
			attribute.String(log.KeyCacheKey, cacheKey),
			attribute.Int("ItemCount", i),
		)
		lg := logger.With().
			Str(log.KeyProcess, "updating product quantity").
			Str(log.KeyCacheKey, cacheKey).
			Int("ItemCount", i).
			Logger()

		msg := fmt.Sprintf(
			"get product quantity from cache for productId=%s",
			item.ProductID.String(),
		)
		logger.Info().Msg(msg)
		span.AddEvent(msg)
		quantityStr, err := redis.JSONGet(c, cacheKey, ".quantity").Result()
		if err != nil {
			err = fmt.Errorf("failed getting quantity from cache with error=%w", err)
			commonErrors.HandleError(err, span)
			lg.Error().Err(err).Msg(err.Error())
			return err
		}
		msg = fmt.Sprintf(
			"got product quantity=%s from cache for productId=%s",
			quantityStr,
			item.ProductID.String(),
		)
		logger.Info().Msg(msg)
		span.AddEvent(msg)

		msg = fmt.Sprintf(
			"converting quantity=%s to int for productId=%s",
			quantityStr,
			item.ProductID.String(),
		)
		lg.Info().Msg(msg)
		span.AddEvent(msg)
		quantity, err := strconv.Atoi(quantityStr)
		if err != nil {
			err = fmt.Errorf("failed converting quantity to int with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return err
		}
		msg = fmt.Sprintf(
			"converted quantity=%d to int for productId=%s",
			quantity,
			item.ProductID.String(),
		)
		lg.Info().Msg(msg)
		span.AddEvent(msg)

		msg = fmt.Sprintf(
			"updating product quantity=%d to database for productId=%s",
			quantity,
			item.ProductID.String(),
		)
		lg.Info().Msg(msg)
		span.AddEvent(msg)
		updated, err := queries.UpdateProductQuantity(
			c,
			repository.UpdateProductQuantityParams{ID: item.ProductID, Quantity: int32(quantity)},
		)
		if err != nil {
			err = fmt.Errorf("failed updating product quantity to database with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return err
		}
		msg = fmt.Sprintf(
			"updated product quantity=%d to database for productId=%s",
			quantity,
			item.ProductID.String(),
		)
		lg.Info().Msg(msg)
		span.AddEvent(msg)
		updatedList[i] = updated
	}
	err = tx.Commit(c)
	if err != nil {
		err = fmt.Errorf("failed updating product quantity with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	logger = logger.With().Any(log.KeyUpdatedProductList, updatedList).Logger()
	logger.Info().Msg("updated product quantity")
	span.AddEvent("updated product quantity")

	return nil
}
