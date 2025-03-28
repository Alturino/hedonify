package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/product/internal/controller"
	"github.com/Alturino/ecommerce/product/internal/otel"
	"github.com/Alturino/ecommerce/product/internal/service"
)

func RunProductService(c context.Context) {
	c, span := otel.Tracer.Start(c, "RunProductService")
	defer span.End()

	cfg := config.Get(c, constants.APP_PRODUCT_SERVICE)

	logger := log.Get(filepath.Join("/var/log/", constants.APP_PRODUCT_SERVICE+".log"), cfg.Application).
		With().
		Str(constants.KEY_APP_NAME, constants.APP_PRODUCT_SERVICE).
		Str(constants.KEY_TAG, "main RunProductService").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Handle("/metrics", promhttp.Handler())
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	shutdownFuncs, err := inOtel.InitOtelSdk(c, constants.APP_PRODUCT_SERVICE, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		logger.Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return
	}
	logger.Info().Msg("initialized otel sdk")
	defer func() {
		logger.Info().Msg("shutting down otel")
		err = inOtel.ShutdownOtel(c, shutdownFuncs)
		if err != nil {
			err = fmt.Errorf("failed shutting down otel with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("shutdown otel")
	}()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	c = logger.WithContext(c)
	db := infra.NewDatabaseClient(c, cfg.Database)
	logger.Info().Msg("initialized database")
	defer func() {
		logger := logger.With().
			Str(constants.KEY_PROCESS, "shutting down database connection").
			Logger()
		logger.Info().Msg("shutting down database connection")
		db.Close()
		logger.Info().Msg("shutdown database connection")
	}()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	c = logger.WithContext(c)
	cache := infra.NewCacheClient(c, cfg.Cache)
	logger.Info().Msg("initialized cache")
	defer func() {
		logger := logger.With().
			Str(constants.KEY_PROCESS, "shutting down cache connection").
			Logger()
		logger.Info().Msg("shutting down cache connection")
		span.AddEvent("shutting down cache connection")
		err := cache.Close()
		if err != nil {
			err = fmt.Errorf("failed closing cache with error=%w", err)
			logger.Error().Err(err).Msg(err.Error())
			inOtel.RecordError(err, span)
			return
		}
		span.AddEvent("shutdown cache connection")
		logger.Info().Msg("shutdown cache connection")
	}()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing productService").Logger()
	logger.Info().Msg("initializing productService")
	queries := repository.New(db)
	productService := service.NewProductService(db, queries, cache)
	logger.Info().Msg("initialized productService")

	logger = logger.With().Str(constants.KEY_PROCESS, "attach product controller").Logger()
	logger.Info().Msg("attaching product controller")
	controller.AttachProductController(mux, &productService)
	logger.Info().Msg("attached product controller")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext: func(net.Listener) context.Context {
			lg := logger.With().
				Reset().
				Timestamp().
				Caller().
				Stack().
				Str(constants.KEY_APP_NAME, constants.APP_PRODUCT_SERVICE).
				Logger()
			c = lg.WithContext(c)
			return c
		},
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	logger.Info().Msg("initialized server")
	defer func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "shutting down http server").Logger()
		span.AddEvent("shutting down http server")
		logger.Info().Msg("shutting down http server")
		err = server.Shutdown(c)
		if err != nil {
			err = fmt.Errorf("failed shutting down http server with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		span.AddEvent("shutdown server")
		logger.Info().Msg("shutdown server")
	}()

	go func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
			err = fmt.Errorf("encounter error=%w while running server", err)
			logger.Error().Err(err).Msg(err.Error())
			c = logger.WithContext(c)
			if err := inOtel.ShutdownOtel(c, shutdownFuncs); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)
				inOtel.RecordError(err, span)
				logger.Error().Err(err).Msg(err.Error())
			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")
}
