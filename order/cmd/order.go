package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/Alturino/ecommerce/internal/common"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
)

func RunOrderService(c context.Context) {
	logger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", common.AppOrderService)).
		With().
		Str(log.KeyAppName, common.AppOrderService).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	cfg := config.InitConfig(c, common.AppProductService)
	logger = logger.With().Any(log.KeyConfig, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	shutdownFuncs, err := otel.InitOtelSdk(c, common.AppOrderService)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(log.KeyProcess, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	router := mux.NewRouter()
	router.Use(middleware.Logging)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(log.KeyProcess, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", "localhost", 8080),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      router,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(log.KeyProcess, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
			err = fmt.Errorf("encounter error=%w while running server", err)
			logger.Error().Err(err).Msg(err.Error())
			if err := otel.ShutdownOtel(c, shutdownFuncs); err != nil {
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
	err = server.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("failed shutting down server with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("shutdown server")

	logger.Info().Msg("shutting down otel")
	c = logger.WithContext(c)
	err = otel.ShutdownOtel(c, shutdownFuncs)
	if err != nil {
		err = fmt.Errorf("failed shutting down otel with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("shutdown otel")

	logger.Info().Msg("server completely shutdown")
}
