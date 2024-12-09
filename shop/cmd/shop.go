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

func RunShopService(c context.Context) {
	logger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", common.AppShopService)).
		With().
		Str(log.KeyAppName, common.AppShopService).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	cfg := config.InitConfig(c, common.AppShopService)
	logger = logger.With().Any(log.KeyConfig, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	shutdownFuncs, err := otel.InitOtelSdk(c, common.AppShopService)
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
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      router,
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
		err = fmt.Errorf("failed shutting down http server with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("shutdown http server")

	logger = logger.With().Str(log.KeyProcess, "shutdown otel").Logger()
	logger.Info().Msg("shutting down otel")
	err = otel.ShutdownOtel(c, shutdownFuncs)
	if err != nil {
		err = fmt.Errorf("failed shutting down otel with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("shutdown otel")

	logger.Info().Msg("completely shutdown server")
}
