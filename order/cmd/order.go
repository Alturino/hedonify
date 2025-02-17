package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
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
	commonOtel "github.com/Alturino/ecommerce/order/internal/common/otel"
	"github.com/Alturino/ecommerce/order/internal/controller"
	"github.com/Alturino/ecommerce/order/internal/service"
	"github.com/Alturino/ecommerce/order/pkg/request"
)

func RunOrderService(c context.Context) {
	c, span := commonOtel.Tracer.Start(c, "RunOrderService")
	defer span.End()

	logger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", constants.APP_ORDER_SERVICE)).
		With().
		Str(log.KEY_APP_NAME, constants.APP_ORDER_SERVICE).
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	cfg := config.InitConfig(c, constants.APP_ORDER_SERVICE)
	logger = logger.With().Any(log.KEY_CONFIG, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	shutdownFuncs, err := otel.InitOtelSdk(c, constants.APP_ORDER_SERVICE, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	defer func() {
		logger.Info().Msg("shutting down otel")
		c = logger.WithContext(c)
		err = otel.ShutdownOtel(c, shutdownFuncs)
		if err != nil {
			err = fmt.Errorf("failed shutting down otel with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("shutdown otel")
	}()
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	database := infra.NewDatabaseClient(c, cfg.Database)
	defer func() {
		logger = logger.With().Str(log.KEY_PROCESS, "closing database").Logger()
		logger.Info().Msg("closing database")
		database.Close()
		logger.Info().Msg("closed database")
	}()
	queries := repository.New(database)
	logger.Info().Msg("initialized database")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	cache := infra.NewCacheClient(c, cfg.Cache)
	defer func() {
		logger = logger.With().Str(log.KEY_PROCESS, "closing cache").Logger()
		logger.Info().Msg("closing cache")
		err = cache.Close()
		if err != nil {
			err = fmt.Errorf("failed closing cache with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("closed cache")
	}()
	logger.Info().Msg("initialized cache")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing order service").Logger()
	logger.Info().Msg("initializing order service")
	c = logger.WithContext(c)
	orderService := service.NewOrderService(database, queries, cache)
	logger.Info().Msg("initialized order service")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Use(otelmux.Middleware(constants.APP_ORDER_SERVICE), middleware.Logging, middleware.Auth)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing order controller").Logger()
	logger.Info().Msg("initializing order controller")
	queue := make(chan request.CreateOrder, 1)
	defer close(queue)
	controller.AttachOrderController(mux, orderService, queue)
	logger.Info().Msg("initializing order controller")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	defer func() {
		logger = logger.With().Str(log.KEY_PROCESS, "shutting down server").Logger()
		logger.Info().Msg("shutting down server")
		err = server.Shutdown(c)
		if err != nil {
			err = fmt.Errorf("failed shutting down server with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("shutdown server")
	}()
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(log.KEY_PROCESS, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(log.KEY_PROCESS, "shutdown server").Logger()
			err = fmt.Errorf("encounter error=%w while running server", err)
			commonErrors.HandleError(err, span)
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

	orderWorker := NewOrderWorker(orderService, queue)
	logger = logger.With().Str(log.KEY_PROCESS, "start-worker").Logger()
	logger.Info().Msg("start order worker")
	span.AddEvent("start order worker")
	var wg sync.WaitGroup
	wg.Add(1)
	c = logger.WithContext(c)
	go orderWorker.StartWorker(c, &wg)
	wg.Wait()

	<-c.Done()
	logger = logger.With().Str(log.KEY_PROCESS, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")
}
