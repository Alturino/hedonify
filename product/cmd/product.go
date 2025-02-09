package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
	commonOtel "github.com/Alturino/ecommerce/product/internal/common/otel"
	"github.com/Alturino/ecommerce/product/internal/controller"
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
	productService := service.NewProductService(db, queries, cache)
	logger.Info().Msg("initialized productService")

	logger = logger.With().Str(log.KeyProcess, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.StrictSlash(true)
	mux.Use(otelmux.Middleware(constants.AppProductService), middleware.Logging)
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

	<-c.Done()
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
