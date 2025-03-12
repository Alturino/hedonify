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
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	"github.com/Alturino/ecommerce/cart/internal/controller"
	"github.com/Alturino/ecommerce/cart/internal/otel"
	"github.com/Alturino/ecommerce/cart/internal/service"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
)

func RunCartService(c context.Context) {
	c, span := otel.Tracer.Start(c, "RunCartService")
	defer span.End()

	cfg := config.Get(c, constants.APP_CART_SERVICE)

	logger := log.Get(filepath.Join("/var/log/", constants.APP_CART_SERVICE+".log"), cfg.Application).
		With().
		Str(constants.KEY_APP_NAME, constants.APP_CART_SERVICE).
		Str(constants.KEY_TAG, "main RunCartService").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Use(
		otelmux.Middleware(constants.APP_CART_SERVICE),
		middleware.Logging,
		middleware.RecoverPanic,
	)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	otelShutdowns, err := inOtel.InitOtelSdk(c, constants.APP_CART_SERVICE, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	defer func() {
		logger.Info().Msg("shutting down otel")
		c = logger.WithContext(c)
		err = inOtel.ShutdownOtel(c, otelShutdowns)
		if err != nil {
			err = fmt.Errorf("failed shutting down otel with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("shutdown otel")
	}()
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	c = logger.WithContext(c)
	db := infra.NewDatabaseClient(c, cfg.Database)
	defer func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "shutting down database").Logger()
		logger.Info().Msg("shutting down database")
		db.Close()
		logger.Info().Msg("shutdown database")
	}()
	logger.Info().Msg("initialized database")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	c = logger.WithContext(c)
	cache := infra.NewCacheClient(c, cfg.Cache)
	defer func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "shutting down cache").Logger()
		logger.Info().Msg("shutting down cache")
		err = cache.Close()
		if err != nil {
			err = fmt.Errorf("failed shutting down cache with error=%w", err)
			logger.Error().Err(err).Msg(err.Error())
			inOtel.RecordError(err, span)
			return
		}
		logger.Info().Msg("shutdown cache")
	}()
	logger.Info().Msg("initialized cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing cart service").Logger()
	logger.Info().Msg("initializing cart service")
	queries := repository.New(db)
	cartService := service.NewCartService(db, queries, cache)
	logger.Info().Msg("initialized cart service")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing cart controller").Logger()
	logger.Info().Msg("initializing cart controller")
	controller.AttachCartController(mux, cartService)
	logger.Info().Msg("initialized cart controller")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext: func(net.Listener) context.Context {
			return logger.WithContext(c)
		},
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)

		logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("error=%w occured while server is running", err)
			logger.Error().Err(err).Msg(err.Error())

			c = logger.WithContext(c)
			if err := inOtel.ShutdownOtel(c, otelShutdowns); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)
				logger.Error().Err(err).Msg(err.Error())
				return
			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")
	logger.Info().Msg("shutting down http server")

	logger = logger.With().Str(constants.KEY_PROCESS, "shutting down http server").Logger()
	logger.Info().Msg("shutting down http server")
	err = server.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("failed shutting down http server with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("shutdown http server")
}
