package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	commonOtel "github.com/Alturino/ecommerce/cart/internal/common/otel"
	"github.com/Alturino/ecommerce/cart/internal/controller"
	"github.com/Alturino/ecommerce/cart/internal/service"
	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
)

func RunCartService(c context.Context) {
	c, span := commonOtel.Tracer.Start(c, "RunCartService")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KEY_APP_NAME, constants.APP_CART_SERVICE).
		Str(log.KEY_TAG, "main RunCartService").
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "init config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	cfg := config.InitConfig(c, constants.APP_CART_SERVICE)
	logger = logger.With().Any(log.KEY_CONFIG, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Use(otelmux.Middleware(constants.APP_CART_SERVICE), middleware.Logging, middleware.Auth)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	otelShutdowns, err := otel.InitOtelSdk(c, constants.APP_CART_SERVICE, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	defer func() {
		logger.Info().Msg("shutting down otel")
		c = logger.WithContext(c)
		err = otel.ShutdownOtel(c, otelShutdowns)
		if err != nil {
			err = fmt.Errorf("failed shutting down otel with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("shutdown otel")
	}()
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	c = logger.WithContext(c)
	db := infra.NewDatabaseClient(c, cfg.Database)
	defer func() {
		logger = logger.With().Str(log.KEY_PROCESS, "shutting down database").Logger()
		logger.Info().Msg("shutting down database")
		db.Close()
		logger.Info().Msg("shutdown database")
	}()
	logger.Info().Msg("initialized database")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	c = logger.WithContext(c)
	cache := infra.NewCacheClient(c, cfg.Cache)
	defer func() {
		logger = logger.With().Str(log.KEY_PROCESS, "shutting down cache").Logger()
		logger.Info().Msg("shutting down cache")
		err = cache.Close()
		if err != nil {
			err = fmt.Errorf("failed shutting down cache with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("shutdown cache")
	}()
	logger.Info().Msg("initialized cache")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing cart service").Logger()
	logger.Info().Msg("initializing cart service")
	queries := repository.New(db)
	cartService := service.NewCartService(db, queries, cache)
	logger.Info().Msg("initialized cart service")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing cart controller").Logger()
	logger.Info().Msg("initializing cart controller")
	controller.AttachCartController(mux, &cartService)
	logger.Info().Msg("initialized cart controller")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	httpServer := http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(log.KEY_PROCESS, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", httpServer.Addr)

		logger = logger.With().Str(log.KEY_PROCESS, "shutdown server").Logger()
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("error=%w occured while server is running", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())

			c = logger.WithContext(c)
			if err := otel.ShutdownOtel(c, otelShutdowns); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)
				commonErrors.HandleError(err, span)
				logger.Error().Err(err).Msg(err.Error())
				return
			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(log.KEY_PROCESS, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")
	logger.Info().Msg("shutting down http server")

	logger = logger.With().Str(log.KEY_PROCESS, "shutting down http server").Logger()
	logger.Info().Msg("shutting down http server")
	err = httpServer.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("failed shutting down http server with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("shutdown http server")
}
