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

	"github.com/Alturino/ecommerce/cart/internal/controller"
	"github.com/Alturino/ecommerce/cart/internal/repository"
	"github.com/Alturino/ecommerce/cart/internal/service"
	"github.com/Alturino/ecommerce/internal/common"
	"github.com/Alturino/ecommerce/internal/config"
	database "github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
)

func RunCartService(c context.Context) {
	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyAppName, common.AppCartService).
		Str(log.KeyTag, "main RunCartService").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "init config").Logger()
	logger.Info().Msg("initializing config")
	cfg := config.InitConfig(c, common.AppCartService)
	logger = logger.With().Any(log.KeyConfig, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Use(middleware.Logging)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(log.KeyProcess, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	otelShutdowns, err := otel.InitOtelSdk(c, common.AppCartService)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(log.KeyProcess, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	db := database.NewDatabaseClient(c, cfg.Database)
	logger.Info().Msg("initialized database")

	logger = logger.With().Str(log.KeyProcess, "initializing cart service").Logger()
	logger.Info().Msg("initializing cart service")
	queries := repository.New(db)
	cartService := service.NewCartService(db, queries)
	logger.Info().Msg("initialized cart service")

	logger = logger.With().Str(log.KeyProcess, "initializing cart controller").Logger()
	logger.Info().Msg("initializing cart controller")
	controller.AttachCartController(mux, &cartService)
	logger.Info().Msg("initialized cart controller")

	logger = logger.With().Str(log.KeyProcess, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(log.KeyProcess, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)

		logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("error=%w occured while server is running", err)
			logger.Error().Err(err).Msg(err.Error())
			if err := otel.ShutdownOtel(c, otelShutdowns); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)
				logger.Error().Err(err).Msg(err.Error())
				return
			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")

	logger.Info().Msg("shutting down http server")
	err = server.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("filed shutting down http server with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("shutdown http server")

	logger.Info().Msg("shutting down otel")
	err = otel.ShutdownOtel(c, otelShutdowns)
	if err != nil {
		err = fmt.Errorf("failed shutting down otel with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("shutdown otel")

	logger.Info().Msg("server completely shutdown")
}
